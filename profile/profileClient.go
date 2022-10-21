package profile

// import (
// 	"fmt"
// 	"os"
// 	"os/exec"
// 	"time"
// )

// // . "github.com/kubearmor/KubeArmor/tests/util"
// type name struct {
// 	resource []string
// 	res      []string
// 	freq     map[string]int
// }

// func clrscr() {
// 	cmd := exec.Command("clear") //Linux example, its tested
// 	cmd.Stdout = os.Stdout
// 	cmd.Run()
// }

// func Start() {
// 	var n name
// 	// var err error
// 	go GetLogs()
// 	for {
// 		// time.Sleep(3 * time.Second)
// 		i := len(Telemetry)
// 		if i <= 0 {
// 			time.Sleep(10 * time.Millisecond)
// 			continue
// 		}
// 		// b, err := json.MarshalIndent(Telemetry, "", "  ")
// 		// if err != nil {
// 		// 	fmt.Println("error:", err)
// 		// }
// 		// fmt.Printf(string(b))
// 		for _, item := range Telemetry {
// 			if item.Operation == "File" {
// 				n.resource = append(n.resource, item.Resource)
// 			}
// 		}

// 		n.freq = make(map[string]int)
// 		for _, name := range n.resource {
// 			n.freq[name] = n.freq[name] + 1
// 		}

// 		for key, _ := range n.freq {
// 			n.res = append(n.res, key)
// 			// fmt.Printf("%s %d\n", key, element)
// 		}
// 		fmt.Printf("%v", n.resource)

// 		// time.Sleep(10 * time.Second)
// 		// break
// 	}

// 	fmt.Printf("%+v", n.res)

// }
