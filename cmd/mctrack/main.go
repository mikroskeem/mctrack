package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/mikroskeem/mcping"
)

type ServerInfo struct {
	Name        string
	CurrentIcon *string
	IP          string
}

type ServerPingResponse struct {
	Name       string
	IP         string
	ResolvedIP string
	Timestamp  time.Time
	Online     int
}

func resolveAddress(addr string, ctx context.Context) (string, error) {
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

	// Try to resolve SRV
	if _, addrs, err := net.DefaultResolver.LookupSRV(ctx, "minecraft", "tcp", resolvedHost); err == nil {
		if len(addrs) > 0 {
			firstAddr := addrs[0]
			resolvedHost = firstAddr.Target
			resolvedPort = int(firstAddr.Port)

			// SRV records usually produce a dot into the end
			if resolvedHost[len(resolvedHost)-1] == '.' {
				resolvedHost = resolvedHost[0 : len(resolvedHost)-1]
			}
		}
	} else if rerr, ok := err.(*net.DNSError); ok && !rerr.IsNotFound {
		return "", err
	}

	if finalHosts, err := net.DefaultResolver.LookupHost(ctx, resolvedHost); err == nil {
		if len(finalHosts) > 0 {
			for _, ip := range finalHosts {
				parsed := net.ParseIP(ip)

				// Skip nil and IPv6 addresses
				if parsed == nil || parsed.To4() == nil {
					continue
				}

				resolvedHost = ip
			}
		} else {
			// TODO: handle earlier!
		}
	} else if rerr, ok := err.(*net.DNSError); ok && !rerr.IsNotFound {
		return "", err
	}

	finalAddr := net.JoinHostPort(resolvedHost, strconv.Itoa(resolvedPort))
	return finalAddr, nil
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
	n := 1
	for {
		resp, err := pingCtx(ctx, address)
		if err != nil {
			if err == context.DeadlineExceeded {
				return nil, err
			} else if err, ok := err.(net.Error); ok && err.Timeout() {
				// Retry
				fmt.Printf("%s ping attempt %d\n", address, n)
				time.Sleep(1500 * time.Millisecond)
				n += 1
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
	respCh := make(chan ServerPingResponse, len(servers))

	for _, info := range servers {
		resolveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resolved, err := resolveAddress(info.IP, resolveCtx)
		if err == context.DeadlineExceeded {
			fmt.Printf("failed to resolve IP for %s: %s", info.Name, err)
			continue
		} else if err != nil {
			panic(err)
		}

		wg.Add(1)

		go func(address string, info ServerInfo) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			defer wg.Done()

			var ts time.Time
			var onlinePlayers int = 0

			resp, err := retryPingCtx(ctx, address)
			if err == context.DeadlineExceeded {
				fmt.Printf("%s did not respond\n", info.Name)
			} else if nerr, ok := err.(net.Error); ok {
				fmt.Printf("%s did not respond: %s\n", info.Name, nerr)
			} else if err != nil {
				// TODO: better handling perhaps
				fmt.Printf("%s did not respond: %s\n", info.Name, nerr)
			} else if err == nil {
				onlinePlayers = resp.Online
				fmt.Printf("%s has %d players online\n", info.Name, onlinePlayers)
			}

			ts = time.Now()
			respCh <- ServerPingResponse{
				info.Name, info.IP, address, ts, onlinePlayers,
			}
		}(resolved, info)
	}

	go func() {
		wg.Wait()
		close(respCh)
	}()

	var responses [][]interface{}
	for resp := range respCh {
		responses = append(responses, []interface{}{
			resp.Name, resp.IP, resp.ResolvedIP, resp.Timestamp, resp.Online,
		})
	}

	fmt.Printf("inserting %d entries\n", len(responses))
	inserted, err := pool.CopyFrom(
		context.Background(),
		pgx.Identifier{"mctrack_servers"},
		[]string{"name", "ip", "resolved_ip", "timestamp", "online"},
		pgx.CopyFromRows(responses),
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("inserted %d entries\n", inserted)
}
