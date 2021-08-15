package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	doh "github.com/mikroskeem/go-doh-client"
	"github.com/mikroskeem/mcping/v2"
)

type ServerInfo struct {
	ID          uint
	Name        string
	CurrentIcon *string
	IP          string
}

type ServerPingResponse struct {
	ID         uint
	Name       string
	IP         string
	ResolvedIP string
	Timestamp  time.Time
	Online     int
}

var (
	dnsClient = doh.Resolver{
		Host:  "1.1.1.1",
		Class: doh.IN,
	}
)

func resolveAddress(addr string, ctx context.Context) (resolved string, normalized string, err error) {
	var resolvedHost string = addr
	var resolvedPort int = 25565

	// Split host & port if present
	if host, port, err := net.SplitHostPort(addr); err == nil {
		resolvedHost = host
		parsedPort, err := strconv.Atoi(port)
		if err != nil {
			return "", "", err
		}
		resolvedPort = parsedPort
	}

	// Build normalized address here before values get overwritten by next DNS queries
	normalized = net.JoinHostPort(resolvedHost, strconv.Itoa(resolvedPort))

	// Try to resolve SRV
	if addrs, _, err := dnsClient.LookupServiceContext(ctx, "minecraft", "tcp", resolvedHost); err == nil {
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
		return "", "", err
	}

	if finalHosts, _, err := dnsClient.LookupAContext(ctx, resolvedHost); err == nil {
		if len(finalHosts) > 0 {
			for _, ip := range finalHosts {
				parsed := net.ParseIP(ip.IP4)

				// Skip nil addresses
				if parsed == nil {
					continue
				}

				resolvedHost = parsed.String()
			}
		} else {
			// TODO: handle earlier!
			log.Printf("no meaningful dns records for %s found", resolvedHost)
			return "", "", doh.ErrNameError
		}
	} else {
		return "", "", err
	}

	resolved = net.JoinHostPort(resolvedHost, strconv.Itoa(resolvedPort))
	return resolved, normalized, nil
}

func retryPingCtx(ctx context.Context, name string, address string, realAddress string) (resp mcping.PingResponse, err error) {
	n := 1
	for {
		if err = ctx.Err(); err != nil {
			return
		}
		resp, err = mcping.Ping(ctx, address, mcping.WithServerAddress(realAddress), mcping.WithReadWriteTimeout(900, 900))
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return
			} else if err, ok := err.(net.Error); ok && err.Timeout() {
				// Retry
				log.Printf("server '%s' (%s) ping attempt %d", name, address, n)
				<-time.After(500 * time.Duration(n) * time.Millisecond)
				n += 1
				continue
			}

			return
		}

		return
	}
}

func main() {
	connString := os.Getenv("MCTRACK_DATABASE_URL")
	if connString == "" {
		panic("MCTRACK_DATABASE_URL is not set")
	}

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	pool, err := pgxpool.ConnectConfig(ctx, poolConfig)
	if err != nil {
		panic(err)
	}

	for {
		log.Println("pinging")
		mainLoop(ctx, pool)
		log.Println("sleeping")
		<-time.After(10 * time.Second)
	}
}

func mainLoop(ctx context.Context, pool *pgxpool.Pool) {
	// Query configured servers
	var servers []ServerInfo
	rows, err := pool.Query(ctx, "SELECT id, name, ip FROM mctrack_watched_servers WHERE last_successful_ping IS NULL OR (now() - last_successful_ping) <= interval '30 days';")
	if err != nil {
		panic(err)
	}

	for rows.Next() {
		var serverID uint
		var serverName string
		var serverIP string
		if err := rows.Scan(&serverID, &serverName, &serverIP); err != nil {
			panic(err)
		}

		servers = append(servers, ServerInfo{serverID, serverName, nil, serverIP})
	}

	if err := rows.Err(); err != nil {
		panic(err)
	}

	rows.Close()

	var wg sync.WaitGroup
	respCh := make(chan *ServerPingResponse, len(servers))

	resolveCtx, resolveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer resolveCancel()

	log.Printf("pinging %d servers", len(servers))
	for _, info := range servers {
		go queryServer(&info, resolveCtx, &wg, respCh)
	}

	go func() {
		wg.Wait()
		close(respCh)
	}()

	var responses [][]interface{}
	responseTsByID := map[uint]time.Time{}
	for resp := range respCh {
		var onlineValue *int = nil
		if resp.Online > -1 {
			onlineValue = &resp.Online
			responseTsByID[resp.ID] = resp.Timestamp
		}
		responses = append(responses, []interface{}{
			resp.Name, resp.IP, resp.ResolvedIP, resp.Timestamp, onlineValue,
		})
	}

	err = pool.BeginTxFunc(ctx, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.CopyFrom(
			ctx,
			pgx.Identifier{"mctrack_servers"},
			[]string{"name", "ip", "resolved_ip", "timestamp", "online"},
			pgx.CopyFromRows(responses),
		)
		if err != nil {
			return err
		}

		// Update last ping status for all servers
		for id, ts := range responseTsByID {
			_, err := tx.Exec(ctx, "UPDATE mctrack_watched_servers SET last_successful_ping = $1 WHERE id = $2", ts, id)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		panic(err)
	}

	successful := len(responseTsByID)
	totalServers := len(servers)

	log.Printf("total %d; resolved=%d, successful=%d (%d diff)", totalServers, len(responses), successful, totalServers-successful)
}

func queryServer(info *ServerInfo, resolveCtx context.Context, wg *sync.WaitGroup, respCh chan<- *ServerPingResponse) {
	// Try to resolve the real IP, and normalize the input
	resolvedAddr, normalizedAddr, err := resolveAddress(info.IP, resolveCtx)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		log.Printf("server '%s' did not respond (dns/timeout): %s", info.Name, err)
		return
	} else if derr, ok := err.(*net.DNSError); ok {
		log.Printf("server '%s' did not respond (dns/error): %s", info.Name, derr)
		return
	} else if err != nil {
		log.Printf("server '%s' did not respond (dns/unk): %s", info.Name, derr)
		return
	}

	// Add itself to the wait group
	wg.Add(1)

	// Now ping the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer wg.Done()

	var onlinePlayers int = -1
	resp, err := retryPingCtx(ctx, info.Name, resolvedAddr, normalizedAddr)
	if nerr, ok := err.(net.Error); ok {
		log.Printf("server '%s' did not respond (net): %s", info.Name, nerr)
	} else if errors.Is(err, io.EOF) {
		log.Printf("server '%s' did not respond (io): %s", info.Name, err)
	} else if err != nil {
		log.Printf("server '%s' did not respond (unk): %s", info.Name, err)
	} else {
		onlinePlayers = resp.Players.Online
	}

	respCh <- &ServerPingResponse{
		info.ID, info.Name, info.IP, resolvedAddr, time.Now(), onlinePlayers,
	}
}
