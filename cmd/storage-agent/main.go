// storage-agent is a Linux host service for ZFS/Btrfs data units, never a public TCP service.
package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"minhanos.local/platform/internal/storage"
)

func main() {
	volumes, err := storage.ParseVolumes(os.Getenv("MINHANOS_STORAGE_VOLUMES"))
	if err != nil {
		log.Fatal("MINHANOS_STORAGE_VOLUMES: ", err)
	}
	socket := os.Getenv("STORAGE_AGENT_SOCKET")
	if socket == "" {
		socket = "/run/minhanos-storage/agent.sock"
	}
	if err = os.MkdirAll(filepath.Dir(socket), 0750); err != nil {
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
	server := &http.Server{Handler: storage.AgentHandler(volumes), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 2 * time.Minute}
	log.Printf("storage-agent ready on Unix socket %s with %d volume(s)", socket, len(volumes))
	log.Fatal(server.Serve(listener))
}
