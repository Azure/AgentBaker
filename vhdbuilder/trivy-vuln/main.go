package main

import "fmt"
/*
multiple reports of vulnerabilities in the package
pack1 pack4 pack4

we need to know the team owner of the vulnerable package
pack1 team1
pack2 team2
pack3 team1
pack4 team3
pack5 team1

we need to group all of the vulnerabilities that a team owns 
icm team1 - pack1, pack5
icm team3 - pack4

we need to get all of the packages from the json's, group them by team, and don't allow duplicate packages
we need to send the vulnrabilities to the teams
	only create the ICM if one does not exist or does not have the newest package 
	ICM1 - pack1, pack5 - not closed then pack3 gets a vuln 
	ICM2 - pack3
	or we just keep updating the existing ICM? - tbd 
*/
func main() {
	fmt.Println("Hello, playground")
}
