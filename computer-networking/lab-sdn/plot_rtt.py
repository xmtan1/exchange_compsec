import re
import matplotlib.pyplot as plt
import os

def get_rtt_from_file(file_path):
    """Helper function to read a file and extract RTT values."""
    try:
        with open(file_path, "r") as f:
            data = f.read()
        # Find all occurrences of the pattern 'time=...' and convert them to float
        return [float(x) for x in re.findall(r"time=(\d+\.\d+)", data)]
    except FileNotFoundError:
        print(f"Warning: The file was not found at {file_path}")
        return [] # Return an empty list if the file doesn't exist

# --- File and Path Setup ---
# Get the current directory where the script is located
current_directory = os.getcwd()

# Define the full paths for your input files and the output plot
h1_h2_path = os.path.join(current_directory, "h1_to_h2.txt")
h2_h1_path = os.path.join(current_directory, "h2_to_h1.txt") # Path for the second file
output_path = os.path.join(current_directory, "RTT_comparison_plot.png")

# --- Data Extraction ---
# Get the RTT values from both files
rtt_h1_h2 = get_rtt_from_file(h1_h2_path)
rtt_h2_h1 = get_rtt_from_file(h2_h1_path)

# Create a common x-axis based on the shorter of the two datasets to avoid errors
num_points = min(len(rtt_h1_h2), len(rtt_h2_h1))
runtime = range(1, num_points + 1)

# Trim the datasets to be the same length
rtt_h1_h2 = rtt_h1_h2[:num_points]
rtt_h2_h1 = rtt_h2_h1[:num_points]

# --- Plotting ---
# Plot the first dataset (h1 to h2)
plt.plot(runtime, rtt_h1_h2, marker='o', color='blue', label='h1 to h2')

# Plot the second dataset (h2 to h1) on the SAME figure
plt.plot(runtime, rtt_h2_h1, marker='x', color='red', label='h2 to h1')

# --- Final Touches ---
# Add titles and labels for clarity
plt.title("RTT Comparison: h1 to h2 vs. h2 to h1")
plt.xlabel("Runtime (Ping Sequence)")
plt.ylabel("RTT (ms)")
plt.grid(True)

# This command is crucial for showing the labels for each line
plt.legend()

# Save the combined plot to a file
plt.savefig(output_path)

print(f"Combined plot saved successfully to: {output_path}")