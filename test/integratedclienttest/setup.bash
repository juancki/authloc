
BUILD_NAME='mainexe'

go build -o $BUILD_NAME main.go

./mainexe -testID=1 -testM=10 | sed 's/.*/111: &/' &
./mainexe -testID=2 -testM=20 | sed 's/.*/2-2: &/' &
./mainexe -testID=3 -testM=30 | sed 's/.*/-3-: &/' &
./mainexe -testID=4 -testM=30 | sed 's/.*/--4: &/' &

for job in `jobs -p`
do
    echo $job
    wait $job || let "FAIL+=1"
done

