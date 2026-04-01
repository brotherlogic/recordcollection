sudo apt update
sudo apt install -y  protobuf-compiler xdg-utils 
/usr/local/go/bin/go install google.golang.org/protobuf/cmd/protoc-gen-go@latest 
/usr/local/go/bin/go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Account for Ghostty
tic -x ghostty.terminfo

# Install tmux and emacs
sudo apt-get update && sudo apt-get install -y tmux emacs

# Install dependencies
/usr/local/go/bin/go get github.com/brotherlogic/godiscogs@latest