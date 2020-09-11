#!/bin/bash
set -e
if [ $# -ne 3 ]; then
	echo "Usage: $0 [labname] [test] [repeat time]"
	exit 1
fi
export "GOPATH=$(git rev-parse --show-toplevel)"
cd "${GOPATH}/src/$1"
logicalCpuCount=$([ $(uname) = 'Darwin' ] && 
                  sysctl -n hw.logicalcpu_max || 
                  lscpu -p | egrep -v '^#' | wc -l)


for ((i=0;i<$3;)); do
	parallelJobCount=$(($3 - $i))
	if [[ $parallelJobCount -ge $logicalCpuCount ]]; then
		parallelJobCount=$logicalCpuCount
	fi	

	echo "Run $parallelJobCount tasks in parallel"

	for ((j=0;j<$parallelJobCount;j++)); do
		time go test -run $2 &
	done

	wait

	i=$(($i + $parallelJobCount))
done
