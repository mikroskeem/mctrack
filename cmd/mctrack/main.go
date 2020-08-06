package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/mikroskeem/mcping"
)

type ServerInfo struct {
	Name        string
	CurrentIcon *string
	IP          string
}

func resolveAddress(addr string) (string, error) {
	var resolvedHost string = addr
	var resolvedPort int = 25565

	// Split host & port if present
	if host, port, err := net.SplitHostPort(addr); err == nil {
		resolvedHost = host
		parsedPort, err := strconv.Atoi(port)
		if err != nil {
			return "", err
		}
		resolvedPort = parsedPort
	}

	if _, addrs, err := net.DefaultResolver.LookupSRV(context.Background(), "minecraft", "tcp", resolvedHost); err == nil {
		if len(addrs) > 0 {
			firstAddr := addrs[0]
			resolvedHost = firstAddr.Target
			resolvedPort = int(firstAddr.Port)

			// SRV records usually produce a dot into the end
			if resolvedHost[len(resolvedHost)-1] == '.' {
				resolvedHost = resolvedHost[0:len(resolvedHost)-1]
			}
		}
	}

	return net.JoinHostPort(resolvedHost, strconv.Itoa(resolvedPort)), nil
}

func pingCtx(ctx context.Context, address string) (*mcping.PingResponse, error) {
	ch := make(chan interface{}, 1)
	go func() {
		res, err := mcping.PingWithTimeout(address, 30*time.Second)
		if err != nil {
			ch <- err
		} else {
			ch <- &res
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if err, ok := res.(error); ok {
			return nil, err
		}
		return res.(*mcping.PingResponse), nil
	}
}

func retryPingCtx(ctx context.Context, address string) (*mcping.PingResponse, error) {
	for {
		resp, err := pingCtx(ctx, address)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				// Retry
				continue
			}

			return nil, err
		}

		return resp, nil
	}
}

func main() {
	connString := "postgres://postgres:longpassword@localhost:5433/mctrack"
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		panic(err)
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), poolConfig)
	if err != nil {
		panic(err)
	}

	// Query configured servers
	var servers []ServerInfo
	rows, err := pool.Query(context.Background(), "select name, ip from mctrack_watched_servers;")
	if err != nil {
		panic(err)
	}

	for rows.Next() {
		var serverName string
		var serverIP string
		if err := rows.Scan(&serverName, &serverIP); err != nil {
			panic(err)
		}

		servers = append(servers, ServerInfo{serverName, nil, serverIP})
	}

	if err := rows.Err(); err != nil {
		panic(err)
	}

	rows.Close()

	var wg sync.WaitGroup
	wg.Add(len(servers))

	for _, info := range servers {
		resolved, err := resolveAddress(info.IP)
		if err != nil {
			panic(err)
		}

		go func(address string, info ServerInfo) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			defer wg.Done()

			var ts time.Time

			if resp, err := retryPingCtx(ctx, address); err != nil {
				if err == context.DeadlineExceeded || err == context.Canceled {
					fmt.Printf("%s did not respond\n", info.Name)
				} else if nerr, ok := err.(net.Error); ok {
					fmt.Printf("%s did not respond: %s\n", nerr)
				} else {
					panic(err)
				}
			} else {
				ts = time.Now()
				fmt.Printf("%s has %d players online\n", info.Name, resp.Online)

				// TODO: CopyFrom
				_, err := pool.Exec(
					context.Background(),
					"insert into mctrack_servers (name, ip, resolved_ip, timestamp, online) values ($1, $2, $3, $4, $5)",
					info.Name, info.IP, address, ts, resp.Online,
				)
				if err != nil {
					panic(err)
				}
			}
		}(resolved, info)
	}

	wg.Wait()
}
