#!/bin/bash
set -ex

protoc --go_out=. *.proto 