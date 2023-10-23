package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/jessevdk/go-flags"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var opts struct {
	Threads      int    `short:"t" long:"threads" default:"8" description:"How many threads should be used"`
	ResolverIP   string `short:"r" long:"resolver" description:"IP of the DNS resolver to use for lookups"`
	ResolverFile string `short:"f" long:"resolverfile" description:"File with list of DNS resolver IPs"`
	Protocol     string `short:"P" long:"protocol" choice:"tcp" choice:"udp" default:"udp" description:"Protocol to use for lookups"`
	Port         uint16 `short:"p" long:"port" default:"53" description:"Port to bother the specified DNS resolver on"`
	Domain       bool   `short:"d" long:"domain" description:"Output only domains"`
}

var resolverIPs []string

func main() {

	_, err := flags.ParseArgs(&opts, os.Args)

	if err != nil {
		os.Exit(1)
	}

	if opts.ResolverFile != "" {
		loadResolversFromFile(opts.ResolverFile)
	}

	numWorkers := opts.Threads

	work := make(chan string)
	go func() {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			work <- s.Text()
		}
		close(work)
	}()

	wg := &sync.WaitGroup{}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go doWork(work, wg)
	}
	wg.Wait()
}

func testResolver(resolverIP string) bool {
	// Create a context that is canceled after 500 milliseconds
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel() // Always call the cancel function to release resources

	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 500 * time.Millisecond, // Set the timeout for the Dialer as well
			}
			return d.DialContext(ctx, opts.Protocol, fmt.Sprintf("%s:%d", resolverIP, opts.Port))
		},
	}

	_, err := r.LookupIP(ctx, "ip", "google.com")
	return err == nil
}

func loadResolversFromFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	var tempResolvers []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		tempResolvers = append(tempResolvers, scanner.Text())
	}

	for _, resolver := range tempResolvers {
		if testResolver(resolver) {
			resolverIPs = append(resolverIPs, resolver)
		} else {

		}
	}

	if len(resolverIPs) == 0 {
		panic("No working resolvers found.")
	}

	rand.New(rand.NewSource(time.Now().UnixNano()))
}

func getRandomResolver() string {
	if len(resolverIPs) == 0 {
		return opts.ResolverIP
	}
	return resolverIPs[rand.Intn(len(resolverIPs))]
}

func doWork(work chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	var r *net.Resolver
	r = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, opts.Protocol, fmt.Sprintf("%s:%d", getRandomResolver(), opts.Port))
		},
	}

	for ip := range work {
		addr, err := r.LookupAddr(context.Background(), ip)
		if err != nil {
			continue
		}

		for _, a := range addr {
			if opts.Domain {
				fmt.Println(strings.TrimRight(a, "."))
			} else {
				fmt.Println(ip, "\t", a)
			}
		}
	}
}
