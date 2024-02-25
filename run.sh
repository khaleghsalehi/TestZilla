#!/bin/bash
clear
# kill old service
kill -9 `ps -aux | grep testzilla | awk '{print $2}'`

#check if default port is Open
netstat  -anpt | grep 9090 | grep LIST
ret=$?
echo $ret
if [[ $ret  -ne 1 ]]; then
  clear
  echo "🦖 :\> Testzilla need to Listen on TCP port 9090. It's available by now"
  exit 1
else
  clear
  echo "🦖 :\> Testzilla port 9090 is ready."
fi
echo "🦖 :\> Are you sure PostgresSQL is installed and username/password already set for Testzilla?"
read -p "Continue? (Y/N): " confirm && [[ $confirm == [yY] || $confirm == [yY][eE][sS] ]] ||  exit 0

echo "🦖 :\> Are you sure Golang is installed too?"
read -p "Continue? (Y/N): " confirm && [[ $confirm == [yY] || $confirm == [yY][eE][sS] ]] ||  exit 0


make clean
make build
echo "Start Testzilla Server, Press any key to continue..."
nohup ./testzilla server &
tail -f nohup.out