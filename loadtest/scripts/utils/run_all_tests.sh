#!/bin/bash

# 负载测试 - 一键运行所有实验

set -e

echo "========================================="
echo "  CS6650 - Load Testing Suite"
echo "========================================="
echo ""
echo "This will run all 4 experiments:"
echo "  1. Connection Scaling (100 connections)"
echo "  2. Throughput Test (500 users, 10 msg/s)"
echo "  3. Fan-out Test (50 users)"
echo "  4. Failure Injection (manual - will be skipped)"
echo ""
echo "Estimated total time: 15-20 minutes"
echo ""
read -p "Press Enter to continue or Ctrl+C to cancel..."

# Create results directory
RESULTS_DIR="results_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"
echo ""
echo "Results will be saved to: $RESULTS_DIR"
echo ""

# Experiment 1
echo ""
echo "========================================="
echo "  Experiment 1: Connection Scaling"
echo "========================================="
python experiment1_connection_scaling.py --connections 100
mv experiment1_results_*.json "$RESULTS_DIR/" 2>/dev/null || true

# Wait between experiments
echo ""
echo "Waiting 10 seconds before next experiment..."
sleep 10

# Experiment 2
echo ""
echo "========================================="
echo "  Experiment 2: Throughput Test"
echo "========================================="
python experiment2_throughput.py --users 500 --msg-per-sec 10 --duration 60
mv experiment2_results_*.json "$RESULTS_DIR/" 2>/dev/null || true

# Wait between experiments
echo ""
echo "Waiting 10 seconds before next experiment..."
sleep 10

# Experiment 3
echo ""
echo "========================================="
echo "  Experiment 3: Fan-out Stress Test"
echo "========================================="
python experiment3_fanout.py --group-size 50 --messages 10
mv experiment3_results_*.json "$RESULTS_DIR/" 2>/dev/null || true

# Summary
echo ""
echo "========================================="
echo "  All Experiments Complete!"
echo "========================================="
echo ""
echo "Results saved in: $RESULTS_DIR"
echo ""
echo "Experiment 4 (Failure Injection) requires manual intervention."
echo "Run it separately:"
echo "  python experiment4_failure.py --users 100 --duration 300"
echo ""
echo "Summary of result files:"
ls -lh "$RESULTS_DIR"
echo ""
