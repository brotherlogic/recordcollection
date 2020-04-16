protoc --proto_path ../../../ -I=./proto --go_out=plugins=grpc:./proto proto/recordcollection.proto
mv proto/github.com/brotherlogic/recordcollection/proto/* ./proto
