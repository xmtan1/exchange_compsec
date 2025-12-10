import subprocess
import time
import os
import signal
import sys

# CONFIGURATION
BINARY = "./chord"
BASE_IP = "127.0.0.1"
BASE_PORT = 4170
NODES = {} # Store process handles: {port: process}

def log(msg):
    print(f"[TEST RUNNER] {msg}")

def start_node(port, join_port=None):
    cmd = [BINARY, "-a", BASE_IP, "-p", str(port), "-ts", "3000", "-tff", "1000", "-tcp", "1000", "-r", "3"]
    
    # Add Join flags if this isn't the first node
    if join_port:
        cmd += ["-ja", BASE_IP, "-jp", str(join_port)]
        
    # Start process with pipes for stdin/stdout
    p = subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=0 # Unbuffered to see logs immediately
    )
    NODES[port] = p
    log(f"Started Node {port} (PID: {p.pid})")
    return p

def send_command(port, command):
    if port not in NODES:
        log(f"Cannot command Node {port}: Node is dead.")
        return
    
    p = NODES[port]
    try:
        log(f"Sending to :{port} -> '{command}'")
        p.stdin.write(command + "\n")
        p.stdin.flush()
    except BrokenPipeError:
        log(f"Node {port} pipe broken.")

def kill_node(port):
    if port in NODES:
        p = NODES[port]
        log(f"⚡ KILLING NODE {port} (Simulating Crash) ⚡")
        p.terminate() # or p.kill() for harder kill
        p.wait()
        del NODES[port]
    else:
        log(f"Node {port} already dead.")

def cleanup():
    log("Cleaning up all nodes...")
    for port, p in NODES.items():
        try:
            p.kill()
            p.wait()
        except Exception as e:
            pass # Process might already be dead
    
    if os.path.exists("secret.txt"):
        try:
            os.remove("secret.txt")
            log("Removed test artifact 'secret.txt'")
        except Exception as e:
            log(f"Warning: Could not remove secret.txt: {e}")

    log("Cleanup complete.")

# --- THE TEST SCENARIO ---
try:
    # 1. SETUP RING
    log("=== PHASE 1: BOOTSTRAP ===")
    p1 = start_node(4170)
    time.sleep(1) # Wait for start
    
    p2 = start_node(4171, join_port=4170)
    time.sleep(1)
    
    p3 = start_node(4172, join_port=4170)
    
    log("Waiting 10s for ring stabilization...")
    time.sleep(10)

    # 2. STORE DATA
    log("\n=== PHASE 2: DATA STORAGE ===")
    # Create a dummy file
    with open("secret.txt", "w") as f:
        f.write("This is critical data that must survive.")
    
    # Instruct Node 4170 to store it
    send_command(4170, "StoreFile secret.txt")
    time.sleep(2) # Wait for replication
    
    # 3. VERIFY REPLICATION
    log("\n=== PHASE 3: VERIFY STATE ===")
    send_command(4170, "dump")
    send_command(4171, "dump")
    send_command(4172, "dump")
    # (You would manually inspect logs here, or we could parse stdout)

    # 4. CHAOS MONKEY (Kill the likely owner)
    log("\n=== PHASE 4: CHAOS (Failover) ===")
    kill_node(4170) # Kill the bootstrap/primary node
    
    log("Waiting 5s for failover detection...")
    time.sleep(5)

    # 5. RETRIEVAL CHECK
    log("\n=== PHASE 5: RETRIEVAL CHECK ===")
    # Ask a survivor (4172) for the file. 
    # Even though 4170 is dead, 4171 or 4172 should have the Replica.
    send_command(4172, "Lookup secret.txt")
    
    # Keep alive briefly to see output
    time.sleep(5)

except KeyboardInterrupt:
    log("Test interrupted.")
finally:
    cleanup()