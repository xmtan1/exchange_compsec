import subprocess
import time
import os
import sys
import shutil

# CONFIGURATION
USE_GO_RUN = False 
BINARY_NAME = "chord"
BASE_IP = "127.0.0.1"
NODES = {} 

def log(msg):
    print(f"\033[96m[TEST RUNNER] {msg}\033[0m") # Cyan color for runner logs

def generate_certs():
    if not os.path.exists("server.crt") or not os.path.exists("server.key"):
        log("Generating TLS certificates for testing...")
        try:
            subprocess.run(
                ["openssl", "req", "-x509", "-newkey", "rsa:4096", 
                 "-keyout", "server.key", "-out", "server.crt", 
                 "-days", "365", "-nodes", "-subj", "/CN=localhost"],
                check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
            )
            log("Certificates generated.")
        except FileNotFoundError:
            log("Error: 'openssl' not found. Please install git-bash or openssl, or generate certs manually.")
            sys.exit(1)

def build_binary():
    if not USE_GO_RUN:
        log("Building Go binary...")
        subprocess.run(["go", "build", "-o", BINARY_NAME], check=True)

def start_node(port, join_port=None):
    cmd = []
    if USE_GO_RUN:
        cmd = ["go", "run", "."]
    else:
        cmd = [f"./{BINARY_NAME}"]

    cmd += ["-a", BASE_IP, "-p", str(port), "-ts", "3000", "-tff", "1000", "-tcp", "1000", "-r", "3"]
    
    if join_port:
        cmd += ["-ja", BASE_IP, "-jp", str(join_port)]
        
    # stdout=sys.stdout direct show the output
    p = subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        stdout=sys.stdout, 
        stderr=sys.stderr,
        text=True,
        bufsize=0 
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
        try:
            p.wait(timeout=2)
        except subprocess.TimeoutExpired:
            p.kill()
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
    # 0. PREPARE
    generate_certs()
    build_binary()

    # 1. SETUP RING
    log("=== PHASE 1: BOOTSTRAP (TLS Enabled) ===")
    start_node(4170)
    time.sleep(2)
    start_node(4171, join_port=4170)
    time.sleep(2)
    start_node(4172, join_port=4170)
    
    log("Waiting 5s for stabilization...")
    time.sleep(5)

    # 2. STORE DATA (Test Encryption + Replication)
    log("\n=== PHASE 2: STORE FILE (Check for Encryption Logs) ===")
    secret_content = "This is Top Secret Data!"
    with open("secret.txt", "w") as f:
        f.write(secret_content)
    
    # Instruct Node 4170 to store it
    send_command(4170, "storefile secret.txt")
    time.sleep(3) 

    # 3. VERIFY STATE
    log("\n=== PHASE 3: DUMP STATE (Check for REPLICAS) ===")
    send_command(4170, "dump")
    send_command(4171, "dump")
    send_command(4172, "dump")
    time.sleep(2)

    # 4. CHAOS (Test Fault Tolerance)
    log("\n=== PHASE 4: KILLING NODE 4170 ===")
    kill_node(4170) # Kill the bootstrap/primary node
    
    log("Waiting 5s for ring to realize node is dead...")
    time.sleep(5)

    # 5. RETRIEVAL (Test Decryption + Failover)
    log("\n=== PHASE 5: RETRIEVAL (Check for Decryption Logs) ===")
    # Ask a survivor (4172) for the file. 
    # Even though 4170 is dead, 4171 or 4172 should have the Replica.
    send_command(4172, "lookup secret.txt")
    
    # Keep alive briefly to see output
    time.sleep(5)

except KeyboardInterrupt:
    log("Test interrupted.")
finally:
    cleanup()