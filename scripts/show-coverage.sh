#!/bin/bash
# Simple script to show coverage percentage
if [ -f "coverage_dev.out" ]; then
    echo "📊 Coverage: $(go tool cover -func=coverage_dev.out | tail -n 1 | awk '{print $3}')"
else
    echo "📊 Coverage: No coverage file found"
fi
