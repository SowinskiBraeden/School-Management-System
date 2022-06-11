#!/usr/bin/python3
from prettytable import PrettyTable
from typing import Tuple
import json
import sys

# Import required utilities
from util.mockStudents import getSampleStudents
from util.generateCourses import getSampleCourses

# Import Algorithm
from scheduleGenerator.generator import generateScheduleV3

def errorOutput(students) -> Tuple[PrettyTable, dict, dict]:
  # Error Table calulation / output  
  f = open('./output/conflicts.json')
  conflicts = json.load(f)
  f.close()
  totalCritical = conflicts["Critical"]["Students"]
  totalAcceptable = conflicts["Acceptable"]["Students"]

  t = PrettyTable(['Type', 'Error %', 'Success %', 'Student Error Ratio'])
  
  errorsC = round(totalCritical / len(students) * 100, 2)
  successC = round(100 - errorsC, 2)
  errorsA = round(totalAcceptable / len(students) * 100, 2)
  successA = round(100 - errorsA, 2)
  
  t.add_row(['Critical', f"{errorsC} %", f"{successC} %", f"{totalCritical}/{len(students)} Students"])
  t.add_row(['Acceptable', f"{errorsA} %", f"{successA} %", f"{totalAcceptable}/{len(students)} Students"])
  
  return t, conflicts["Critical"], conflicts["Acceptable"]

if __name__ == '__main__':
  
  if len(sys.argv) == 1:
    print("Missing argument")
    exit()

  if sys.argv[1].upper() == 'V1':
    print("Processing...")

    mockStudents = generateMockStudents(400)
    timetable = {}
    timetable["Version"] = 1
    timetable["timetable"] = generateScheduleV1(mockStudents, mockCourses)
  
  elif sys.argv[1].upper() == 'V2':
    print("Processing...")
  
    mockStudents = generateMockStudents(400)
    timetable = {}
    timetable["Version"] = 2
    timetable["timetable"] = generateScheduleV2(mockStudents, mockCourses)
  

  elif sys.argv[1].upper() == 'V3':
  
    print("Processing...\n")
  
    sampleStudents = getSampleStudents("./sample_data/course_selection_data.csv", True)
    samplemockCourses = getSampleCourses("./sample_data/course_selection_data.csv", True)
    timetable = {}
    timetable["Version"] = 3
    timetable["timetable"] = generateScheduleV3(sampleStudents, samplemockCourses, 40, "./output/students.json", "./output/conflicts.json")

    errors, _, _ = errorOutput(sampleStudents)
    print(errors)

  elif sys.argv[1].upper() == "ERRORS":
    f = open('./output/students.json')
    studentData = json.load(f)
    f.close()
    errors, critical, acceptable = errorOutput(studentData)
    print()
    print(errors)

    print(f"\n{critical['Total']} critical errors")
    for i in range(len(critical["Errors"])):
      print(f"x{critical['Errors'][i]['Total']} {critical['Errors'][i]['Code']} Errors: Critical - {critical['Errors'][i]['Description']}")

    print(f"\n{acceptable['Total']} acceptable errors")
    for i in range(len(acceptable["Errors"])):
      print(f"x{acceptable['Errors'][i]['Total']} {acceptable['Errors'][i]['Code']} Errors: Critical - {acceptable['Errors'][i]['Description']}")

    exit()

  else:
    print("Invalid argument")
    exit()

  with open("./output/timetable.json", "w") as outfile:
    json.dump(timetable, outfile, indent=2)

  print("\nDone")
