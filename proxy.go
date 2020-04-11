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
	"io"
	"net"
	"time"
)

// Proxy object declaration
type Proxy struct {
	logger     *Logger
	err        chan error
	rcvd, sent int64
}

// GetProxy returns a new Proxy object
func GetProxy(log *Logger) *Proxy {
	return &Proxy{
		logger: log,
		err:    make(chan error),
	}
}

// Transfer forwards data between two Conn objects
func (p *Proxy) Transfer(lconn net.Conn, rconn net.Conn) int64 {
	// Disable the deadline with a zero value
	var deadline time.Time
	err := rconn.SetDeadline(deadline)
	if err != nil {
		p.logger.Warningf("SetDeadline failed: %s", err)
	}
	err = lconn.SetDeadline(deadline)
	if err != nil {
		p.logger.Warningf("SetDeadline failed: %s", err)
	}

	p.logger.Debugf("Forwarding data")
	go p.copy(rconn, lconn, &p.sent)
	go p.copy(lconn, rconn, &p.rcvd)

	// Wait for the 1st copying direction
	err = <-p.err
	if err == nil {
		p.logger.Debugf("1st copying direction success")
	} else {
		p.logger.Warningf("1st copying direction failed: %s", err)
	}

	// Set a deadline for the 2nd copying direction
	deadline = time.Now().Add(time.Minute)
	err = rconn.SetDeadline(deadline)
	if err != nil {
		p.logger.Warningf("SetDeadline failed: %s", err)
	}
	err = lconn.SetDeadline(deadline)
	if err != nil {
		p.logger.Warningf("SetDeadline failed: %s", err)
	}

	// Wait for the 2nd copying direction
	err = <-p.err
	if err == nil {
		p.logger.Debugf("2nd copying direction success")
	} else {
		p.logger.Warningf("2nd copying direction failed: %s", err)
	}

	p.logger.Infof("Closed: %d bytes sent, %d bytes recieved", p.sent, p.rcvd)
	return p.sent + p.rcvd
}

func (p *Proxy) copy(dst io.Writer, src io.Reader, bytes *int64) {
	n, err := io.Copy(dst, src)
	*bytes += n
	if err == nil {
		if conn, ok := dst.(*net.TCPConn); ok {
			_ = conn.CloseWrite() // Send TCP FIN
		} else if conn, ok := dst.(*tls.Conn); ok {
			_ = conn.CloseWrite() // Send close_notify
		}
	} else {
		if conn, ok := dst.(*net.TCPConn); ok {
			_ = conn.SetLinger(0) // Reset the dst socket
		}
		if conn, ok := src.(*net.TCPConn); ok {
			_ = conn.SetLinger(0) // Reset the src socket
		}
	}
	p.err <- err
}

// vim: noet:ts=4:sw=4:sts=4:spell
