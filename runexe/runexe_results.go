package main

import (
	"runlib/subprocess"
	"fmt"
	"strconv"
)

type Verdict int

const (
	SUCCESS = Verdict(0)
	FAIL = Verdict(1)
	CRASH = Verdict(2)
	TIME_LIMIT_EXCEEDED = Verdict(3)
	MEMORY_LIMIT_EXCEEDED = Verdict(4)
	IDLE = Verdict(5)
	SECURITY_VIOLATION = Verdict(6)
)

func (v Verdict) String() string {
	switch v {
	case SUCCESS:
		return "SUCCESS"
	case FAIL:
		return "FAIL"
	case CRASH:
		return "CRASH"
	case TIME_LIMIT_EXCEEDED:
		return "TIME_LIMIT_EXCEEDED"
	case MEMORY_LIMIT_EXCEEDED:
		return "MEMORY_LIMIT_EXCEEDED"
	case IDLE:
		return "IDLENESS_LIMIT_EXCEEDED"
	case SECURITY_VIOLATION:
		return "SECURITY_VIOLATION"
	}
	return "CRASH"
}

func GetVerdict(r *subprocess.SubprocessResult) Verdict {
	switch {
	case r.SuccessCode == 0:
		return SUCCESS
	case r.SuccessCode & (subprocess.EF_PROCESS_LIMIT_HIT | subprocess.EF_PROCESS_LIMIT_HIT_POST) != 0:
		return SECURITY_VIOLATION
	case r.SuccessCode & (subprocess.EF_INACTIVE | subprocess.EF_TIME_LIMIT_HARD) != 0:
		return IDLE
	case r.SuccessCode & (subprocess.EF_TIME_LIMIT_HIT | subprocess.EF_TIME_LIMIT_HIT_POST) != 0:
		return TIME_LIMIT_EXCEEDED
	case r.SuccessCode & (subprocess.EF_MEMORY_LIMIT_HIT | subprocess.EF_MEMORY_LIMIT_HIT_POST) != 0:
		return MEMORY_LIMIT_EXCEEDED
	default:
		return CRASH
	}
	return CRASH
}


func PrintResult(s *subprocess.Subprocess, r *subprocess.SubprocessResult, c string) {
	v := GetVerdict(r)
	fmt.Println("<?xml version = \"1.1\" encoding = \"UTF-8\"?>")
	fmt.Println()
	fmt.Println("<invocationResult>")
	fmt.Println("    <invocationVerdict>" + v.String() + "</invocationVerdict>")
	fmt.Println("    <exitCode>" +
			strconv.Itoa(int(r.ExitCode)) +
			"</exitCode>")
	fmt.Println("    <processorUserModeTime>" +
			strconv.Itoa(int(r.UserTime / 1000)) +
			"</processorUserModeTime>")
	fmt.Println("    <processorKernelModeTime>" +
			strconv.Itoa(int(r.KernelTime / 1000)) +
			"</processorKernelModeTime>")
	fmt.Println("    <passedTime>" +
			strconv.Itoa(int(r.WallTime / 1000)) +
			"</passedTime>")
	fmt.Println("    <consumedMemory>" +
			strconv.Itoa(int(r.PeakMemory)) +
			"</consumedMemory>");
	fmt.Println("    <comment>" + c +
			"</comment>");
	fmt.Println("</invocationResult>")
}
