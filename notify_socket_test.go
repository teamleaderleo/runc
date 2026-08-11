package main

import (
	"bytes"
	"io"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestNotifySocketRunReadyOrder(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
	}{
		{name: "ready first", payload: "READY=1\nSTATUS=ok"},
		{name: "ready second", payload: "STATUS=warming\nREADY=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			hostAddr := net.UnixAddr{Name: dir + "/host.sock", Net: "unixgram"}
			host, err := net.ListenUnixgram("unixgram", &hostAddr)
			if err != nil {
				t.Fatal(err)
			}
			defer host.Close()

			notifyAddr := net.UnixAddr{Name: dir + "/notify.sock", Net: "unixgram"}
			notify, err := net.ListenUnixgram("unixgram", &notifyAddr)
			if err != nil {
				t.Fatal(err)
			}
			defer notify.Close()

			s := &notifySocket{socket: notify, host: hostAddr.Name}
			runChan := make(chan error, 1)
			go func() {
				runChan <- s.run(os.Getpid())
			}()

			sender, err := net.DialUnix("unixgram", nil, &notifyAddr)
			if err != nil {
				t.Fatal(err)
			}
			defer sender.Close()

			if _, err := sender.Write([]byte(tc.payload)); err != nil {
				t.Fatal(err)
			}

			if err := host.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			expectRead(t, host, "READY=1\n")
			expectRead(t, host, "MAINPID="+strconv.Itoa(os.Getpid())+"\n")
			if err := host.SetReadDeadline(time.Time{}); err != nil {
				t.Fatal(err)
			}
			expectBarrier(t, host, runChan)
		})
	}
}

func TestNotifySocketRunIgnoresDatagramWithoutReady(t *testing.T) {
	dir := t.TempDir()
	hostAddr := net.UnixAddr{Name: dir + "/host.sock", Net: "unixgram"}
	host, err := net.ListenUnixgram("unixgram", &hostAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()

	notifyAddr := net.UnixAddr{Name: dir + "/notify.sock", Net: "unixgram"}
	notify, err := net.ListenUnixgram("unixgram", &notifyAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer notify.Close()

	s := &notifySocket{socket: notify, host: hostAddr.Name}
	runChan := make(chan error, 1)
	go func() {
		runChan <- s.run(-1)
	}()

	sender, err := net.DialUnix("unixgram", nil, &notifyAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	if _, err := sender.Write([]byte("STATUS=warming\nERRNO=0")); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-runChan:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("notifySocket.run did not exit after watched PID disappeared")
	}

	if err := host.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var buf [1024]byte
	if n, err := host.Read(buf[:]); err == nil {
		t.Fatalf("unexpected host notification: %q", buf[:n])
	} else if nerr, ok := err.(net.Error); !ok || !nerr.Timeout() {
		t.Fatal(err)
	}
}

// TestNotifyHost tests how runc reports container readiness to the host (usually systemd).
func TestNotifyHost(t *testing.T) {
	addr := net.UnixAddr{
		Name: t.TempDir() + "/testsocket",
		Net:  "unixgram",
	}

	server, err := net.ListenUnixgram("unixgram", &addr)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	client, err := net.DialUnix("unixgram", nil, &addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// run notifyHost in a separate goroutine
	notifyHostChan := make(chan error)
	go func() {
		notifyHostChan <- notifyHost(client, []byte("READY=42"), 1337)
	}()

	// mock a host process listening for runc's notifications
	expectRead(t, server, "READY=42\n")
	expectRead(t, server, "MAINPID=1337\n")
	expectBarrier(t, server, notifyHostChan)
}

func expectRead(t *testing.T, r io.Reader, expected string) {
	var buf [1024]byte
	n, err := r.Read(buf[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf[:n], []byte(expected)) {
		t.Fatalf("Expected to read '%s' but runc sent '%s' instead", expected, buf[:n])
	}
}

func expectBarrier(t *testing.T, conn *net.UnixConn, notifyHostChan <-chan error) {
	var msg, oob [1024]byte
	n, oobn, _, _, err := conn.ReadMsgUnix(msg[:], oob[:])
	if err != nil {
		t.Fatal("Failed to receive BARRIER message", err)
	}
	if !bytes.Equal(msg[:n], []byte("BARRIER=1")) {
		t.Fatalf("Expected to receive 'BARRIER=1' but got '%s' instead.", msg[:n])
	}

	fd := mustExtractFd(t, oob[:oobn])

	// Test whether notifyHost actually honors the barrier
	timer := time.NewTimer(500 * time.Millisecond)
	select {
	case <-timer.C:
		// this is the expected case
		break
	case <-notifyHostChan:
		t.Fatal("runc has terminated before barrier was lifted")
	}

	// Lift the barrier
	err = unix.Close(fd)
	if err != nil {
		t.Fatal(err)
	}

	// Expect notifyHost to terminate now
	err = <-notifyHostChan
	if err != nil {
		t.Fatal("notifyHost function returned with error", err)
	}
}

func mustExtractFd(t *testing.T, buf []byte) int {
	cmsgs, err := unix.ParseSocketControlMessage(buf)
	if err != nil {
		t.Fatal("Failed to parse control message", err)
	}

	fd := 0
	seenScmRights := false
	for _, cmsg := range cmsgs {
		if cmsg.Header.Type != unix.SCM_RIGHTS {
			continue
		}
		if seenScmRights {
			t.Fatal("Expected to see exactly one SCM_RIGHTS message, but got a second one")
		}
		seenScmRights = true
		fds, err := unix.ParseUnixRights(&cmsg)
		if err != nil {
			t.Fatal("Failed to parse SCM_RIGHTS message", err)
		}
		if len(fds) != 1 {
			t.Fatal("Expected to read exactly one file descriptor, but got", len(fds))
		}
		fd = fds[0]
	}
	if !seenScmRights {
		t.Fatal("Control messages didn't contain an SCM_RIGHTS message")
	}

	return fd
}
