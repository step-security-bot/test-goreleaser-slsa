// Copyright 2024 Ajabep
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcpproxy

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/ajabep/test-goreleaser-slsa/internal/configuration"
)

type proxy struct {
	from      string
	to        string
	tlsConfig *tls.Config
}

func newProxy(from, to string, tlsConfig *tls.Config) *proxy {

	return &proxy{
		from:      from,
		to:        to,
		tlsConfig: tlsConfig,
	}
}

func (p *proxy) start(ctx context.Context) error {
	/// Start the proxy. Is blocking!

	listener, err := net.Listen("tcp", p.from)
	if err != nil {
		return err
	}
	defer listener.Close() // nolint

	for {
		select {

		default:
			if connection, err := listener.Accept(); err == nil {
				go p.handle(ctx, connection)
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func (p *proxy) handle(ctx context.Context, connection net.Conn) {

	defer connection.Close() // nolint
	remote, err := tls.Dial("tcp", p.to, p.tlsConfig)
	if err != nil {
		return
	}
	defer remote.Close() // nolint

	subctx, cancel := context.WithCancel(ctx)
	go p.copy(subctx, cancel, remote, connection)
	go p.copy(subctx, cancel, connection, remote)

	<-subctx.Done()
}

func (p *proxy) copy(ctx context.Context, cancel context.CancelFunc, from, to net.Conn) {

	defer cancel()

	var n int
	var err error
	buffer := make([]byte, 1024)

	select {

	default:

		for {
			n, err = to.Read(buffer)
			if err != nil {
				return
			}

			_, err = from.Write(buffer[:n])
			if err != nil {
				return
			}
		}

	case <-ctx.Done():
		return
	}
}

// Start starts the proxy
func Start(cfg *configuration.Configuration, tlsConfig *tls.Config) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := newProxy(cfg.ListenAddress, cfg.Backend, tlsConfig).start(ctx); err != nil {
			log.Fatalln("Unable to start proxy:", err)
		}
	}()

	log.Printf("MTLSProxy is ready. mode:%s listen:%s backend:%s ", cfg.Mode, cfg.ListenAddress, cfg.Backend)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	cancel()
}
