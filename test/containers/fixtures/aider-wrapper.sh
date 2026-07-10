#!/bin/bash
# Aider wrapper for /proc cmdline visibility
exec python3 -c "
import time, sys
sys.argv = ['aider', '--model', 'gpt-4']
time.sleep(86400)
"
