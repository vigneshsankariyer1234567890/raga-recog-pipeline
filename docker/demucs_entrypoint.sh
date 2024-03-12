#!/bin/bash
set -e

cmd="python3 -m demucs -n ${MODEL} --out /data/output"

# Check if GPU is enabled
if [ "$GPU" = "true" ]; then
    cmd="${cmd} --gpus all"
fi

# Check if MP3 output is enabled
if [ "$MP3OUTPUT" = "true" ]; then
    cmd="${cmd} --mp3"
fi

# Check if a split track option is set
if [ -n "$SPLITTRACK" ]; then
    cmd="${cmd} --two-stems ${SPLITTRACK}"
fi

# Add the track to the command
cmd="${cmd} /data/input/$1"

# Execute the command
echo "Running command: $cmd"
eval $cmd
