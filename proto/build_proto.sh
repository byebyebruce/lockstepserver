#!/bin/bash
DIR="$( cd "$( dirname "$0"  )" && pwd  )"
echo pwd=$DIR

function xrsh_get_osname()
{
    uname -s
}

xrsh_get_osname

get_char()  
{  
  SAVEDSTTY=`stty -g`  
  stty -echo  
  stty raw  
  dd if=/dev/tty bs=1 count=1 2> /dev/null  
  stty -raw  
  stty echo  
  stty $SAVEDSTTY  
}  

if [ $(uname -s) = 'Linux' ]; then
	PROTOC=$DIR/../../../deps/protobuf/bin/protoc
else
	PROTOC=$DIR/../../../tools/protoc_win/protoc.exe
fi
