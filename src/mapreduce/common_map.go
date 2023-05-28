package mapreduce

import (
	"hash/fnv"
	"fmt"
	"io/ioutil"
	"os"
	"log"
	"encoding/json"
)

func doMap(
	jobName string, // the name of the MapReduce job
	mapTask int, // which map task this is
	inFile string,
	nReduce int, // the number of reduce task that will be run ("R" in the paper)
	mapF func(filename string, contents string) []KeyValue,
) {
	data,err :=ioutil.ReadFile(inFile)
	if err !=nil{
	    fmt.Println("File reading error",err)
	    return
	}
	kvResult :=mapF(inFile,string(data))
	intermediateFiles :=make([]*os.File,nReduce)
        for i :=0; i<nReduce; i++{
            intermediateFiles[i],err=os.Create(reduceName(jobName,mapTask,i))
            if err !=nil{
                log.Fatal("create file failed",reduceName(jobName,mapTask,i))
       }
     }
       for  _,kv:=range kvResult{
           enc := json.NewEncoder(intermediateFiles[ihash(kv.Key)%nReduce])
           err :=enc.Encode(&kv)
           if err !=nil{
               log.Fatal("encode kv failed",kv)
           }
       }
       for _,file :=range intermediateFiles{
           file.Close()
       }
}

func ihash(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32() & 0x7fffffff)
}
