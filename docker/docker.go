package main

import (
	"flag"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/rcli"
	"github.com/dotcloud/docker/term"
	"io"
	"log"
	"os"
)

func main() {
	if docker.SelfPath() == "/sbin/init" {
		// Running in init mode
		docker.SysInit()
		return
	}
	// FIXME: Switch d and D ? (to be more sshd like)
	flDaemon := flag.Bool("d", false, "Daemon mode")
	flDebug := flag.Bool("D", false, "Debug mode")
	flag.Parse()
	rcli.DEBUG_FLAG = *flDebug
	if *flDaemon {
		if flag.NArg() != 0 {
			flag.Usage()
			return
		}
		if err := daemon(); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := runCommand(flag.Args()); err != nil {
			log.Fatal(err)
		}
	}
}

func daemon() error {
	service, err := docker.NewServer()
	if err != nil {
		return err
	}
	return rcli.ListenAndServe("tcp", "127.0.0.1:4242", service)
}

func runCommand(args []string) error {
	var oldState *term.State
	var err error
	if term.IsTerminal(0) && os.Getenv("NORAW") == "" {
		oldState, err = term.MakeRaw(0)
		if err != nil {
			return err
		}
		defer term.Restore(0, oldState)
	}
	// FIXME: we want to use unix sockets here, but net.UnixConn doesn't expose
	// CloseWrite(), which we need to cleanly signal that stdin is closed without
	// closing the connection.
	// See http://code.google.com/p/go/issues/detail?id=3345
	if conn, err := rcli.Call("tcp", "127.0.0.1:4242", args...); err == nil {
		receiveStdout := docker.Go(func() error {
			_, err := io.Copy(os.Stdout, conn)
			return err
		})
		sendStdin := docker.Go(func() error {
			_, err := io.Copy(conn, os.Stdin)
			if err := conn.CloseWrite(); err != nil {
				log.Printf("Couldn't send EOF: " + err.Error())
			}
			return err
		})
		if err := <-receiveStdout; err != nil {
			return err
		}
		if !term.IsTerminal(0) {
			if err := <-sendStdin; err != nil {
				return err
			}
		}
	} else {
		service, err := docker.NewServer()
		if err != nil {
			return err
		}
		if err := rcli.LocalCall(service, os.Stdin, os.Stdout, args...); err != nil {
			return err
		}
	}
	if oldState != nil {
		term.Restore(0, oldState)
	}
	return nil
}
