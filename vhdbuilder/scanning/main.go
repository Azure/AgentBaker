package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args
	if len(args) > 1 {
		fmt.Println("Argument passed to main.go:", args[1])
		// Use args[1] as needed in your Go program
	} else {
		fmt.Println("No argument passed to main.go")
	}
	vhdImageUrlName := args[0]
	fmt.Println("vhd_name_for_download_url:", vhdImageUrlName)

	// need to call the database to get the n-2 (3) latest vhd versions
	// need to run scanning on 
}

/*
func generate_csv() {
	file, err := os.Create("vhd_versions_in_prod-generated-in-go.csv")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()
	writer := bufio.NewWriter(file)

	// call the kusto query

	err = writer.Flush()
	if err != nil {
		fmt.Println("Error flushing writer:", err)
		return
	}
}

func process_csv() {
	file, err := os.Open("vhd_versions_in_prod.csv")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	reader.Comma = ';'
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		return
	}

	var allVHDS []VHDImages
	for _, row := range records {
		if strings.Contains(row[0], "1604") {
			continue
		}
		if strings.Contains(row[0], "AKSSecurityPatchedVHD") {
			continue
		}
		if strings.Contains(row[0], "uki") {
			continue
		}

		csv_row := strings.Split(row[0], ",")
		version := strings.Trim(csv_row[1], `"`)
		parts := strings.Split(csv_row[0], "/")
		if len(parts) < 2 {
			continue
		}

		imageType := ImageType{OS_SKU: parts[0], Image_Name: parts[2]}

		var found bool
		for i := range allVHDS {
			if allVHDS[i].Version == version {
				allVHDS[i].Images = append(allVHDS[i].Images, imageType)
				found = true
				break
			}
		}

		if !found {
			vhd := VHDImages{
				Version: version,
				Images:  []ImageType{imageType},
			}
			allVHDS = append(allVHDS, vhd)
		}

	}

	file2, err := os.Create("verify_vhds.txt")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file2.Close()
	writer := bufio.NewWriter(file2)
	for _, vhd := range allVHDS {
		fmt.Fprintln(writer, vhd.Version)
		for _, image := range vhd.Images {
			fmt.Fprintf(writer, "    %s %s\n", image.OS_SKU, image.Image_Name)
		}
		fmt.Fprintln(writer)
	}
	err = writer.Flush()
	if err != nil {
		fmt.Println("Error flushing writer:", err)
		return
	}
}
*/

/*
query for csv -
AgentPoolSnapshot
| where PreciseTimeStamp > ago(4h)
| extend nodeImageReference = parse_json(agentPoolVersionProfile).nodeImageReference
| extend id = tostring(nodeImageReference.id)
| where not(id contains "AKSWindows") and not(id contains "1804") and not(id contains "MarinerAKSSig") and not(id contains "MyTestGalleryEastUS")
| extend unique_id = extract(@"galleries/([^/]+/[^/]+/[^/]+/versions/[^/]+)", 1, id)
| extend version = extract(@"versions/([^/]+)", 1, id)
| sort by version asc
| distinct unique_id, version

get the last 3 version values
AgentPoolSnapshot
| where PreciseTimeStamp > ago(4h)
| extend nodeImageReference = parse_json(agentPoolVersionProfile).nodeImageReference
| extend id = tostring(nodeImageReference.id)
| where not(id contains "AKSWindows") and not(id contains "1804") and not(id contains "MarinerAKSSig") and not(id contains "MyTestGalleryEastUS")
| extend unique_id = extract(@"galleries/([^/]+/[^/]+/[^/]+/versions/[^/]+)", 1, id)
| extend version = extract(@"versions/([^/]+)", 1, id)
| distinct version
| sort by version desc
| take 3
*/
