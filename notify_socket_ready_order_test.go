package main

import (
	"net"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

func startNotifyTestProcess(t *testing.T) int {
	t.Helper()

	cmd := exec.Command("sleep", "0.5")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	go func() {
		_ = cmd.Wait()
	}()
	return pid
}

func TestNotifySocketReadyOrder(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		wantReady bool
	}{
		{
			name:      "ready-first",
			payload:   "READY=1\nSTATUS=ok",
			wantReady: true,
		},
		{
			name:      "ready-second",
			payload:   "STATUS=ok\nREADY=1",
			wantReady: true,
		},
		{
			name:      "no-ready",
			payload:   "STATUS=ok",
			wantReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			hostAddr := net.UnixAddr{
				Name: dir + "/host.sock",
				Net:  "unixgram",
			}
			hostServer, err := net.ListenUnixgram("unixgram", &hostAddr)
			if err != nil {
				t.Fatal(err)
			}
			defer hostServer.Close()

			notifyAddr := net.UnixAddr{
				Name: dir + "/notify.sock",
				Net:  "unixgram",
			}
			notifyServer, err := net.ListenUnixgram("unixgram", &notifyAddr)
			if err != nil {
				t.Fatal(err)
			}
			defer notifyServer.Close()

			sender, err := net.DialUnix("unixgram", nil, &notifyAddr)
			if err != nil {
				t.Fatal(err)
			}
			defer sender.Close()

			s := &notifySocket{
				socket:     notifyServer,
				host:       hostAddr.Name,
				socketPath: notifyAddr.Name,
			}
			pid1 := startNotifyTestProcess(t)
			runDone := make(chan error, 1)
			go func() {
				runDone <- s.run(pid1)
			}()

			if _, err := sender.Write([]byte(tt.payload)); err != nil {
				t.Fatal(err)
			}

			if !tt.wantReady {
				select {
				case err := <-runDone:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("notifySocket.run did not return after watched process exited")
				}

				if err := hostServer.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
					t.Fatal(err)
				}
				var buf [128]byte
				if _, err := hostServer.Read(buf[:]); err == nil {
					t.Fatal("unexpected host notification without READY field")
				} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
					t.Fatalf("unexpected host socket read error: %v", err)
				}
				return
			}

			if err := hostServer.SetReadDeadline(time.Now().Add(1500 * time.Millisecond)); err != nil {
				t.Fatal(err)
			}
			expectRead(t, hostServer, "READY=1\n")
			expectRead(t, hostServer, "MAINPID="+strconv.Itoa(pid1)+"\n")
			expectBarrier(t, hostServer, runDone)
		})
	}
}
