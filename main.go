/*
 *  b4ck-client
 *  Copyright 2020 Michał Trojnara

 *  This program is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.

 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.

 *  You should have received a copy of the GNU General Public License
 *  along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

type Context struct {
	raddr     string
	laddr     string
	port      int
	key       []byte
	connID    chan uint64
	logger    *Logger
	tlsConfig *tls.Config
}

func main() {
	// Initialize configuration
	c := GetContext()

	// Spawn a pool of workers
	rand.Seed(time.Now().UnixNano())
	for i := 2; i > 0; i-- {
		go c.worker(c.logger.Child(fmt.Sprintf("%d", i)))
		time.Sleep(time.Duration(900+rand.Int31n(200)) * time.Millisecond)
	}
	c.worker(c.logger.Child("0"))
}

func GetContext() *Context {
	confRaddr := flag.String("r", "", "remote address (mandatory)")
	confLaddr := flag.String("l", ":80", "local address")
	confKey := flag.String("k", "", "authentication key (mandatory)")
	confDebug := flag.String("d", "INFO", "log verbosity")
	confNoTLS := flag.Bool("t", false, "disable TLS (debugging only)")
	flag.Parse()

	// Initialize logging
	logger := GetLogger("b4ck")
	if level, ok := ParseLevel(*confDebug); ok {
		logger.SetLogLevel(level)
	} else {
		logger.SetLogLevel(DEBUG)
	}
	// logger.Infof("%s", logger.EffectiveLogLevel().String())

	// Check for mandatory flags
	mandatory := []string{"r", "k"}
	seen := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { seen[f.Name] = true })
	for _, req := range mandatory {
		if !seen[req] {
			logger.Errorf("Missing mandatory -%s parameter", req)
			os.Exit(2) // exit code used by flag.Parse
		}
	}

	// Split *confRaddr into raddr and port
	t := strings.Split(*confRaddr, ":")
	raddr := strings.Join(t[:len(t)-1], ":") + ":1"
	var err error
	port, err := net.LookupPort("tcp", t[len(t)-1])
	if err != nil {
		logger.Errorf("Port lookup failed: %s", err)
		os.Exit(1)
	}

	// Decode the authentication key
	key, err := base64.RawStdEncoding.DecodeString(*confKey)
	if err != nil {
		logger.Errorf("Invalid key: %s", err)
		os.Exit(1)
	}
	if len(key) != 6 {
		logger.Errorf("Invalid decoded key length: %d", len(key))
		os.Exit(1)
	}

	c := &Context{
		raddr:  raddr,
		laddr:  *confLaddr,
		port:   port,
		key:    key,
		logger: logger,
		connID: make(chan uint64),
	}
	go func() {
		var id uint64
		for {
			c.connID <- id
			id++
		}
	}()

	// Setup TLS configuration
	if !*confNoTLS {
		c.tlsConfig = &tls.Config{
			ServerName:         "free.b4ck.net",
			MinVersion:         tls.VersionTLS13,
			ClientSessionCache: tls.NewLRUClientSessionCache(32),
		}
	}

	c.logger.Infof("Proxying %s->%s", *confRaddr, *confLaddr)
	return c
}

func (c *Context) worker(logger *Logger) {
	for {
		delay := c.remote(false)
		if delay != 0 {
			ms := 1000 + rand.Intn(delay*1000)
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
	}
}

// returns delay in seconds minus 1
func (c *Context) remote(fast bool) int {
	var logger *Logger
	if fast {
		logger = c.logger.Child("fast")
	} else {
		logger = c.logger.Child("slow")
	}

	// Dial rconn
	rconn, err := net.Dial("tcp", c.raddr)
	if err != nil {
		logger.Warningf("Remote connection failed: %s", err)
		return 9
	}
	ropen := true
	defer func() {
		if ropen {
			rconn.Close()
		}
	}()
	err = rconn.SetDeadline(time.Now().Add(time.Minute))
	if err != nil {
		logger.Warningf("SetDeadline failed: %s", err)
		return 99
	}

	// Negotiate TLS
	if c.tlsConfig == nil {
		logger.Debugf("New TCP connection")
	} else {
		conn := tls.Client(rconn, c.tlsConfig)
		err = conn.Handshake() // Needed for ConnectionState()
		if err != nil {
			logger.Warningf("TLS handshake failed: %s", err)
			return 9
		}
		state := conn.ConnectionState()
		v := state.Version
		version := fmt.Sprintf("TLSv%d.%d", v>>8-2, v&255-1)
		if state.DidResume {
			logger.Debugf("New %s connection (resumed session)", version)
		} else {
			logger.Infof("New %s connection (new session)", version)
		}
		rconn = conn
	}

	// Send an authentication request
	err = SndMsg(rconn, &Msg{Type: "listen", Port: c.port, Key: c.key})
	if err != nil {
		logger.Warningf("Failed to send port number: %s", err)
		return 9
	}

	// Process server messages
	for {
		message, err := RcvMsg(rconn)
		if err != nil {
			logger.Warningf("Failed to receive message: %s", err)
			return 9
		}
		switch message.Type {
		case "start":
			ropen = false // rconn will be closed by local()
			go c.local(logger, message, rconn)
			if fast { // Keep a steady pool of fast connections
				go c.remote(true)
			}
			return 0
		case "keepalive":
			logger.Debugf("Received KEEPALIVE")
			if fast {
				err = SndMsg(rconn, &Msg{Type: "info", Text: "TIMEOUT"})
				if err != nil {
					logger.Warningf("Failed to send TIMEOUT: %s", err)
				}
				return 0
			} else {
				err = SndMsg(rconn, &Msg{Type: "keepalive"})
				if err != nil {
					logger.Warningf("Failed to send KEEPALIVE: %s", err)
					return 9
				}
				err = rconn.SetDeadline(time.Now().Add(time.Minute))
				if err != nil {
					logger.Warningf("SetDeadline failed: %s", err)
					return 9
				}
			}
		case "debug":
			logger.Debugf("%s", message.Text)
			return 0
		case "info":
			logger.Infof("%s", message.Text)
			return 0
		case "warning":
			logger.Warningf("%s", message.Text)
			return 0
		case "error":
			logger.Errorf("%s", message.Text)
			os.Exit(1)
		default:
			logger.Warningf("Ignored message: %s: %s", message.Type, message.Text)
		}
	}
}

func (c *Context) local(logger *Logger, message *Msg, rconn net.Conn) {
	defer rconn.Close()

	// Spawn an additional goroutines, ignore the result
	if message.Fast {
		go c.remote(true)
		go c.remote(true)
	}

	// Use a dynamically generated connection id for further logs
	logger = logger.Child(fmt.Sprintf("%d", <-c.connID))
	if message.Fast {
		logger.Infof("Fast connection received from %s", message.Addr)
	} else {
		logger.Infof("Slow connection received from %s", message.Addr)
	}

	// Dial lconn
	logger.Infof("Connecting local service")
	lconn, err := net.Dial("tcp", c.laddr)
	if err != nil {
		logger.Warningf("Local connection failed: %s", err)
		return
	}
	defer lconn.Close()
	err = lconn.SetDeadline(time.Now().Add(time.Minute))
	if err != nil {
		logger.Warningf("SetDeadline failed: %s", err)
		return
	}

	// Send SUCCESS
	err = SndMsg(rconn, &Msg{Type: "success"})
	if err != nil {
		logger.Warningf("Failed to send SUCCESS: %s", err)
		return
	}

	// Forward the data
	p := GetProxy(logger)
	p.Transfer(rconn, lconn)
}

// vim: noet:ts=4:sw=4:sts=4:spell
