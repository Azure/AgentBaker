package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

type VHDImages struct {
	Version string
	Images  []ImageType
}

type ImageType struct {
	OS_SKU     string // AKSUbuntu
	Image_Name string // V2gen2arm64
}

func main() {

	vhdmap := map[ImageType]string{
		{OS_SKU: "AKSCBLMariner", Image_Name: "V2"}:                  "CBLMarinerV2",
		{OS_SKU: "AKSAzureLinux", Image_Name: "V2"}:                  "AzureLinuxV2",
		{OS_SKU: "AKSCBLMariner", Image_Name: "V2gen2"}:              "CBLMarinerV2Gen2",
		{OS_SKU: "AKSAzureLinux", Image_Name: "V2gen2"}:              "AzureLinuxV2Gen2",
		{OS_SKU: "AKSCBLMariner", Image_Name: "V2gen2arm64"}:         "CBLMarinerV2Gen2Arm64",
		{OS_SKU: "AKSAzureLinux", Image_Name: "V2gen2arm64"}:         "AzureLinuxV2Gen2Arm64",
		{OS_SKU: "AKSCBLMariner", Image_Name: "V2katagen2"}:          "CBLMarinerV2kataGen2",
		{OS_SKU: "AKSAzureLinux", Image_Name: "V2katagen2"}:          "AzureLinuxV2kataGen2",
		{OS_SKU: "AKSCBLMariner", Image_Name: "V2gen2TL"}:            "CBLMarinerV2TLGen2",
		{OS_SKU: "AKSAzureLinux", Image_Name: "V2gen2TL"}:            "AzureLinuxV2TLGen2",
		{OS_SKU: "AKSUbuntu", Image_Name: "2004fipscontainerd"}:      "2004",
		{OS_SKU: "AKSUbuntu", Image_Name: "2204gen2arm64containerd"}: "2204Gen2Arm64",
		{OS_SKU: "AKSUbuntu", Image_Name: "2204containerd"}:          "2204",
		{OS_SKU: "AKSUbuntu", Image_Name: "2204gen2containerd"}:      "2204Gen2",
		{OS_SKU: "AKSUbuntu", Image_Name: "2004gen2CVMcontainerd"}:   "2004CVMGen2",
		{OS_SKU: "AKSUbuntu", Image_Name: "2204gen2TLcontainerd"}:    "2204TLGen2",
		// {OS_SKU: "AKSCBLMariner", Image_Name: "V2fips"}: "CBLMarinerV2", // fips can ignore ??
		// {OS_SKU: "AKSAzureLinux", Image_Name: "V2gen2fips"}: "AzureLinuxV2", // fips can ignore?
		// {OS_SKU: "AKSCBLMariner", Image_Name: "V2gen2fips"}: "CBLMarinerV2Gen2",// fips can ignore?
		// {OS_SKU: "AKSAzureLinux", Image_Name: "??"}: "AzureLinuxV2",
		// {OS_SKU: "", Image_Name: ""}: "",mariner gen2 fips
		// {OS_SKU: "AKSUbuntu", Image_type: "2004gen2fipscontainerd"}: "2004Gen2",
	}
	linterannoy(vhdmap)

	generate_csv()
	process_csv()
}

func linterannoy(x map[ImageType]string) {
}

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
*/
