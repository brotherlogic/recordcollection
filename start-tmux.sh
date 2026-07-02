#!/bin/bash

# Ensure the 'recordcollection' session exists
if ! tmux has-session -t recordcollection 2>/dev/null; then
  # Create a new session named 'recordcollection', detached
  cd /workspaces/recordcollection
  tmux new-session -d -s recordcollection
 fi
