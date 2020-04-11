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
	// "fmt"
	"io"

	"github.com/json-iterator/go"
)

var json = jsoniter.ConfigFastest

type Msg struct {
	Type string
	Text string `json:",omitempty"`
	Port int    `json:",omitempty"`
	Key  []byte `json:",omitempty"`
	Fast bool   `json:",omitempty"`
	Addr string `json:",omitempty"`
}

func RcvMsg(r io.Reader) (*Msg, error) {
	var m Msg
	length := make([]byte, 1)
	_, err := io.ReadAtLeast(r, length, len(length))
	if err != nil {
		return &m, err
	}
	serialized := make([]byte, length[0])
	_, err = io.ReadAtLeast(r, serialized, int(length[0]))
	if err != nil {
		return &m, err
	}
	err = json.Unmarshal(serialized, &m)
	return &m, err
}

func SndMsg(w io.Writer, m *Msg) error {
	serialized, err := json.Marshal(m)
	if err != nil {
		return err
	}
	// fmt.Println(string(serialized))
	length := []byte{byte(len(serialized))}
	_, err = w.Write(append(length, serialized...))
	return err
}

// vim: noet:ts=4:sw=4:sts=4:spell
