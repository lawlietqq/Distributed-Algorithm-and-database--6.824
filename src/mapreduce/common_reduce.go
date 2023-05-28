package mapreduce
import (
	"os"
	"sort"
	"log"
	"encoding/json"
)
func doReduce(
	jobName string, // the name of the whole MapReduce job
	reduceTask int, // which reduce task this is
	outFile string, // write the output here
	nMap int, // the number of map tasks that were run ("M" in the paper)
	reduceF func(key string, values []string) string,
) {   
        keyValues :=make(map[string][]string)
        for i:=0; i<nMap; i++{
            file,err :=os.Open(reduceName(jobName,i,reduceTask))//reading the zhongjianwenjian
            if err !=nil{
                log.Fatal("create intermediate file failed",reduceName(jobName,i,reduceTask))
            }
            var kv KeyValue
            dec :=json.NewDecoder(file)
            err = dec.Decode(&kv)
            for err==nil {
                keyValues[kv.Key] = append(keyValues[kv.Key],kv.Value)
                err = dec.Decode(&kv)
            }
            file.Close()
          }
         var keys []string
         for k := range keyValues{
             keys = append(keys,k)
         }
          sort.Strings(keys)
          out, err :=os.Create(outFile)
          if err !=nil{
          log.Fatal("failed to create outfile",outFile)
          }
          enc :=json.NewEncoder(out)
          for _,k :=range keys{
              v :=reduceF(k,keyValues[k])
              err = enc.Encode(KeyValue{ Key:k, Value:v})
              if err !=nil{
                  log.Fatal("failed to encode",KeyValue{Key:k,Value:v})
              }
          }
          out.Close()
}
