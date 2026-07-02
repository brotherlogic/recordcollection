#!/bin/zsh

export GOPATH=/go
export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

sudo apt update
sudo apt install -y  protobuf-compiler xdg-utils 
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest 
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Account for Ghostty
tic -x ghostty.terminfo

# Install tmux and emacs
sudo apt-get install -y tmux emacs

git config --global user.email 'brotherlogic-automation@gmail.com'
git config --global user.name 'Brotherlogic Automation'

# Install ACLI
curl -fsSL https://antigravity.google/cli/install.sh | bash

# Setup Tmux
TMUX_BLOCK=$(cat << 'EOF'
if [ -z "$TMUX" ] && [ -n "$PS1" ]; then
  cd /workspaces/recordcollection
  /workspaces/recordcollection/start-tmux.sh && tmux attach-session -t recordcollection
fi
EOF
)

grep -q "tmux attach-session" ~/.zshrc || echo "$TMUX_BLOCK" >> ~/.zshrc
grep -q "tmux attach-session" ~/.bashrc || echo "$TMUX_BLOCK" >> ~/.bashrc

# Ensure the session is created
/workspaces/recordcollection/start-tmux.sh
