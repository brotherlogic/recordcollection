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
sudo apt-get update && sudo apt-get install -y tmux emacs

git config --global user.email 'brotherlogic-automation@gmail.com'
git config --global user.name 'Brotherlogic Automation'