// files-agent is a Linux host service, never a public TCP service.
package main

import (
	"log"
	"minhanos.local/platform/internal/drive"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	socket := os.Getenv("FILES_AGENT_SOCKET")
	if socket == "" {
		socket = "/run/minhanos-files/agent.sock"
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0750); err != nil {
		log.Fatal(err)
	}
	if fi, err := os.Lstat(socket); err == nil {
		if fi.Mode()&os.ModeSocket == 0 {
			log.Fatal("socket path occupied by non-socket")
		}
		if err = os.Remove(socket); err != nil {
			log.Fatal(err)
		}
	}
	syscall.Umask(0077)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	if err = os.Chown(socket, 0, 10001); err != nil {
		log.Fatal(err)
	}
	if err = os.Chmod(socket, 0660); err != nil {
		log.Fatal(err)
	}
	root, err := os.OpenRoot("/")
	if err != nil {
		log.Fatal(err)
	}
	defer root.Close()
	server := &http.Server{Handler: drive.AgentHandler(root), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Minute, WriteTimeout: 5 * time.Minute}
	log.Print("files-agent ready on Unix socket ", socket)
	log.Fatal(server.Serve(listener))
}
