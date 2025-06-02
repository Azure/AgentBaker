# importing the required modules 
import csv 
import os
import xml.etree.ElementTree as ET 
  
def parseXML(xmlfile): 
    # create element tree object 
    tree = ET.parse(xmlfile) 
  
    # get root element 
    root = tree.getroot() 
  
    for key in root.attrib:
        if key == 'tests':
            testnum = int(root.attrib[key])
        elif key == 'failures':
            failednum = int(root.attrib[key])
        elif key == 'disabled':
            skipednum = int(root.attrib[key])

    runnum = testnum - skipednum

    print('Total: ' + str(testnum) + '  Passed: ' + str(runnum - failednum) + '  Failed: ' + str(failednum) + '  Skipped: ' + str(skipednum))
    print('Total time elapsed: ' + root.attrib['time'])
    
    # create empty list for news items
    newitems = []

    # only one test suite
    testsuite = root.find('./testsuite')

    timestamp = testsuite.attrib['timestamp']
    # iterate news items 
    for testcase in testsuite.findall('./testcase'): 
        # to be sync with traditional kusto table
        item = {
            'TIMESTAMP' : timestamp,
            'ResourceGroup' : resourcegroup,
            'ContainerRuntime' : 'contianerd',
            #'testvmsize' : vmsize,
            'Name' : testcase.attrib['name'],
            'ClassName' : testclass, #testcase.attrib['classname'],
            'Status' : testcase.attrib['status'],
            'RunTime' : testcase.attrib['time'],
            'Level': 4,
            'Failure' : '',
            'SystemError' : '',
        }

        if testcase.attrib['status'] == 'failed':
            for failure in testcase.findall('./failure'):
                item['Failure'] += failure.text
                item['Failure'] += '\n'

            item['Level'] = 2

            for systemerror in testcase.findall('./system-err'):
                item['SystemError'] += systemerror.text
                item['SystemError'] += '\n'

        elif testcase.attrib['status'] == 'skipped':
            for skipped in testcase.findall('./skipped'):
                message = skipped.attrib['message']
                if message != "skipped":
                    item['Failure'] += skipped.attrib['message']
                    item['Failure'] += '\n'

        if item['Name'] in {"[SynchronizedBeforeSuite]", "[SynchronizedAfterSuite]", "[ReportBeforeSuite]", "[ReportAfterSuite] Kubernetes e2e suite report", "[DeferCleanup (Suite)]"}:
            continue
        newitems.append(item)
    
    return newitems

def savetoCSV(newsitems, filename): 
  
    # specifying the fields for csv file 
    fields = ['TIMESTAMP', 'ResourceGroup', 'ContainerRuntime', 'Name', 'ClassName', 'Status', 'RunTime', 'Level', 'Failure', 'SystemError' ] 
  
    # writing to csv file 
    with open(filename, 'w') as csvfile: 
  
        # creating a csv dict writer object 
        writer = csv.DictWriter(csvfile, fieldnames = fields) 
  
        # writing headers (field names) 
        writer.writeheader() 
  
        # writing data rows 
        writer.writerows(newsitems) 

# Get the environment variables
resourcegroup = os.environ.get('RESOURCE_GROUP')
#vmsize = os.environ.get('VM_SIZE')
xmlfile = os.environ.get('TEST_RESULT')
testclass = os.environ.get('TEST_SUITES')
csvfile = os.environ.get('TEST_RESULT_CSV')

def main(): 

    # parse xml file 
    newsitems = parseXML(xmlfile) 
  
    # store news items in a csv file 
    savetoCSV(newsitems, csvfile) 
      
      
if __name__ == "__main__": 
  
    # calling main function 
    main() 