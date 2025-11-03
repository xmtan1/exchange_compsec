package main

import (
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"testing"
)

func TestServerConnectionLimit(t *testing.T) {
	// 1. Setup: Start the server on a random free port
	// We use ":0" to let the OS pick an available port.
	s, err := newServer(":0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	s.Start()
	// Ensure the server is stopped when the test finishes
	defer s.Stop()

	// Get the actual address the listener is using
	serverAddr := s.listener.Addr().String()
	log.Printf("Test server started on %s", serverAddr)

	// 2. Setup Test Parameters
	const numClients = 15 // More than MAX_CONNECTION

	// These atomics are the core of our test.
	// They are safe to use from all client goroutines.
	var activeConnections atomic.Int32 // Tracks current concurrent connections
	var maxConcurrent atomic.Int32     // Tracks the *peak* concurrency we saw
	var totalServed atomic.Int32       // Tracks total clients served

	var wg sync.WaitGroup
	wg.Add(numClients)

	// 3. Run Test: Launch all clients
	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			defer wg.Done()

			// Dial the server
			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				t.Errorf("Client %d failed to dial: %v", clientID, err)
				return
			}
			defer conn.Close()

			// --- THIS IS THE FIX ---
			// Wait for the server to actually handle us.
			// We read the first message. This will BLOCK
			// until a worker goroutine actually runs
			// handleConnection() and sends us the data.
			buf := make([]byte, 1024)
			_, err = conn.Read(buf)
			if err != nil && err != io.EOF {
				t.Errorf("Client %d failed to read welcome msg: %v", clientID, err)
				return
			}
			// --- END FIX ---

			// --- Concurrency Check (Start) ---
			// NOW we can safely say this connection is "active".
			current := activeConnections.Add(1)
			for {
				m := maxConcurrent.Load()
				if current > m {
					if maxConcurrent.CompareAndSwap(m, current) {
						break
					}
				} else {
					break
				}
			}
			// --- Concurrency Check (End) ---

			// Wait for the server to finish its 5-second work
			// and close the connection (which returns EOF).
			_, err = io.ReadAll(conn)
			if err != nil && err != io.EOF {
				t.Errorf("Client %d read error: %v", clientID, err)
			}

			// We are done
			activeConnections.Add(-1)
			totalServed.Add(1)
		}(i)
	}

	// 4. Wait & Verify
	wg.Wait() // Wait for all 15 clients to finish

	log.Printf("Test complete. Max concurrent connections reached: %d", maxConcurrent.Load())
	log.Printf("Test complete. Total clients served: %d", totalServed.Load())

	// **Verification 1: Were all clients served?**
	if totalServed.Load() != numClients {
		t.Errorf("Not all clients were served! Expected %d, got %d",
			numClients, totalServed.Load())
	}

	// **Verification 2: Was the limit respected?**
	// We expect it to hit *exactly* the max, but check <= just in case.
	if maxConcurrent.Load() > MAX_CONNECTION {
		t.Errorf("Connection limit breached! Max was %d, expected %d",
			maxConcurrent.Load(), MAX_CONNECTION)
	}

	// **Verification 3: Did it actually use the pool?**
	// This confirms the pool was saturated.
	if maxConcurrent.Load() != MAX_CONNECTION {
		t.Errorf("Connection limit was not reached! Max was %d, expected %d",
			maxConcurrent.Load(), MAX_CONNECTION)
	}
}
