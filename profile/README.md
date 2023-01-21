## Profiling of Kubearmor Logs using karmor

`karmor profile` shows a Terminal user interface table of three different Operations going on in Kubearmor: Process, File and Network. It maintains a counter of each operation that is happening within the cluster, along with other useful details. It directly fetches data from the `karmor log` API and displays all the needed information. The TUI includes simple navigation between operations and a user input based filter as well. 

![Profile](https://user-images.githubusercontent.com/23097199/213850468-2462e8b2-b4f6-491f-a174-42d217cbfd28.gif)
