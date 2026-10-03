// This synthetic program is compiled and executed only inside the approved sandbox.
package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func must(ok bool, message string) {
	if !ok {
		panic(message)
	}
	fmt.Println("ok:", message)
}
func read(p string) string {
	b, e := os.ReadFile(p)
	if e != nil {
		panic(e)
	}
	return strings.TrimSpace(string(b))
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "oom" {
		var blocks [][]byte
		for {
			b := make([]byte, 16<<20)
			for i := range b {
				b[i] = 1
			}
			blocks = append(blocks, b)
			runtime.KeepAlive(blocks)
		}
	}
	must(os.Getuid() == 65534, "non-root")
	must(os.Getenv("AFTER_HOST_SECRET") == "" && os.Getenv("AWS_SECRET_ACCESS_KEY") == "" && os.Getenv("DOCKER_HOST") == "", "no ambient credentials")
	for _, p := range []string{"/root/.ssh/id_rsa", "/root/.aws/credentials", "/var/run/docker.sock", "/host", "/Users"} {
		_, e := os.Stat(p)
		must(e != nil, "no host path "+p)
	}
	for _, p := range []string{"/input/probe.go", "/input/injected", "/etc/injected"} {
		e := os.WriteFile(p, []byte("attack"), 0600)
		must(e != nil, "read-only "+p)
	}
	must(os.WriteFile("/work/owned", []byte("ok"), 0600) == nil, "owned scratch writable")
	status := read("/proc/self/status")
	must(strings.Contains(status, "NoNewPrivs:\t1"), "no new privileges")
	must(strings.Contains(status, "Seccomp:\t2"), "seccomp filter")
	must(strings.Contains(status, "CapEff:\t0000000000000000"), "no capabilities")
	must(read("/sys/fs/cgroup/memory.max") == "1073741824", "memory hard limit")
	must(read("/sys/fs/cgroup/memory.swap.max") == "0", "no swap")
	must(read("/sys/fs/cgroup/pids.max") == "128", "process hard limit")
	must(read("/sys/fs/cgroup/cpu.max") == "100000 100000", "CPU hard limit")
	interfaces, e := net.Interfaces()
	must(e == nil && len(interfaces) == 1 && interfaces[0].Name == "lo", "loopback-only network")
	for _, address := range []string{"1.1.1.1:443", "192.168.5.2:80", "172.17.0.1:80", "host.docker.internal:80"} {
		c, e := net.DialTimeout("tcp", address, time.Second)
		if c != nil {
			c.Close()
		}
		must(e != nil, "network blocked "+address)
	}
	// The same private namespace permits a real standard-library HTTP exchange.
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	must(e == nil, "HTTP listen")
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 32))
		w.Write(append([]byte("echo:"), body...))
	}), ReadHeaderTimeout: time.Second}
	go server.Serve(listener)
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	response, e := client.Post("http://"+listener.Addr().String(), "text/plain", bytes.NewBufferString("offline"))
	must(e == nil, "HTTP request")
	body, e := io.ReadAll(response.Body)
	response.Body.Close()
	server.Close()
	must(e == nil && string(body) == "echo:offline", "HTTP response")
	// Exercise the kernel process limit with a finite number of sleeping children.
	var children []*exec.Cmd
	denied := false
	for i := 0; i < 140; i++ {
		c := exec.Command("/bin/sleep", "30")
		if c.Start() != nil {
			denied = true
			break
		}
		children = append(children, c)
	}
	for _, c := range children {
		c.Process.Kill()
		c.Wait()
	}
	must(denied, "process exhaustion denied")
	must(!strings.HasSuffix(read("/sys/fs/cgroup/pids.events"), " 0"), "kernel recorded process denial")
	fmt.Println("PROOF PASSED")
}
