# !/bin/bash

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <test-case-data-folder>"
    exit 1;
fi

rm $1/line*.sh
file="./$1/CustomData"
#echo "Processing $file"
lineNumber=`grep "content: \!\!binary"  -n $file | cut -d':' -f1`
for i in $lineNumber; do
c=$((i+1));
#echo "Working on line $c";
z=`sed -n ${c}p $file`

echo $z | base64 --decode | gunzip > $1/line${c}.sh
done