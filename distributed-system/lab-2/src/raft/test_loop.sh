#!/bin/bash

# Usage: ./test_loop.sh [TestPattern] [Count]
#
# Arguments:
#   TestPattern: The regex pattern for the test (e.g., 2A, 2B, 2C, 2D, or TestFigure8)
#   Count:       Number of times to run the test (default is 10)
#
# Examples:
#   ./test_loop.sh 2A 20        (Run Part A 20 times)
#   ./test_loop.sh 2D 50        (Run Part D 50 times)
#   ./test_loop.sh TestFigure8 100  (Run specific test 100 times)

PATTERN=$1
COUNT=$2

# Check if pattern argument is provided
if [ -z "$PATTERN" ]; then
    echo "Error: No test pattern provided."
    echo "Usage: ./test_loop.sh [TestPattern] [Count]"
    echo "Example: ./test_loop.sh 2D 20"
    exit 1
fi

# Default count to 10 if not provided
if [ -z "$COUNT" ]; then
    COUNT=10
fi

echo "--------------------------------------------------"
echo "Starting stress test for '$PATTERN'"
echo "Iterations: $COUNT"
echo "Race Detection: ENABLED (-race)"
echo "--------------------------------------------------"

for i in $(seq 1 $COUNT); do
    # Print progress (using printf to stay on same line until result)
    printf "Run %-3d/%-3d ... " "$i" "$COUNT"

    # Run the test
    # 2>&1 redirects stderr to stdout so we capture panic messages too
    OUTPUT=$(go test -race -run "$PATTERN" 2>&1)
    EXIT_CODE=$?

    if [ $EXIT_CODE -eq 0 ]; then
        echo "✅ PASS"
    else
        echo "❌ FAIL"
        echo ""
        echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
        echo "FAILED on iteration $i"
        echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
        echo ""
        echo "Failure Log Output:"
        echo "--------------------------------------------------"
        echo "$OUTPUT"
        echo "--------------------------------------------------"
        echo "Stopping loop due to failure."
        exit 1
    fi
done

echo ""
echo "🎉 Congratulations! Passed all $COUNT iterations for '$PATTERN'."