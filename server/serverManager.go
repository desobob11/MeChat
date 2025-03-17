package main
func main(){
	port := 9999
	for i := 0; i <2; i++{
		port += i
		go spawn_server(i, port) // spawn 2 servers
	}

	select {}
}