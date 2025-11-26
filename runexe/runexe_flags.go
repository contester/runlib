package main

import (
	"fmt"
	"strconv"
	"strings"
)

type processAffinityFlag uint64

func (t *processAffinityFlag) String() string {
	return "0" + strconv.FormatUint(uint64(*t), 2)
}

func (t *processAffinityFlag) Set(v string) error {
	if len(v) == 0 {
		return nil
	}

	base := 10
	if v[0] == '0' {
		base = 2
		v = v[1:]
	}

	r, err := strconv.ParseUint(v, base, 64)
	if err != nil {
		return err
	}
	*t = processAffinityFlag(r)
	return nil
}

type timeLimitFlag uint64

func (t *timeLimitFlag) String() string {
	return strconv.Itoa(int(*t/1000)) + "ms"
}

func (t *timeLimitFlag) Set(v string) error {
	v = strings.ToLower(v)
	if strings.HasSuffix(v, "ms") {
		r, err := strconv.Atoi(v[:len(v)-2])
		if err != nil {
			return err
		}
		if r < 0 {
			return fmt.Errorf("Invalid time limit %s", v)
		}
		*t = timeLimitFlag(r * 1000)
		return nil
	}
	if strings.HasSuffix(v, "s") {
		v = v[:len(v)-1]
	}
	r, err := strconv.ParseFloat(v, 32)
	if err != nil {
		return err
	}
	if r < 0 {
		return fmt.Errorf("Invalid time limit %s", v)
	}
	*t = timeLimitFlag(r * 1000000)
	return nil
}

type memoryLimitFlag uint64

func (t *memoryLimitFlag) String() string {
	return strconv.FormatUint(uint64(*t), 10)
}

func (t *memoryLimitFlag) Set(v string) error {
	v = strings.ToUpper(v)
	m := 1
	switch v[len(v)-1] {
	case 'M':
		m = 1024 * 1024
	case 'K':
		m = 1024
	case 'G':
		m = 1024 * 1024 * 1024
	}
	if m != 1 {
		v = v[:len(v)-1]
	}
	r, err := strconv.Atoi(v)
	if err != nil {
		return err
	}
	if r < 0 {
		return fmt.Errorf("Invalid memory limit %s", v)
	}
	*t = memoryLimitFlag(r * m)
	return nil
}

type envFlag []string

func (t *envFlag) String() string {
	return strings.Join(*t, "|")
}

func (t *envFlag) Set(v string) error {
	*t = append(*t, v)
	return nil
}

func printUsage() {
	fmt.Printf("runexe 2.0 version %s build %s\n", version, buildid)
	fmt.Println(usageText)
}

const usageText = `
This program runs other program(s) for given period of time with specified
restrictions.

Command line format:
  runexe [<global options>] [<process properties>] program [<parameters>]

Global options:
  -help         - show help
  -xml          - print result in xml format (otherwise, use human-readable)
  -show-kernel-mode-time - include kernel-mode time in human-readable format
                  (always included in xml)
  -x            - return process exit code
  -logfile=<f>  - for runexe developers only
  -interactor="<process properties> interactor <parameters>"
                  INTERACTOR MODE
    Launch another process and cross-connect its stdin&stdout with the main
    program. Inside this flag, you can specify any process-controlling flags:
    interactor can have its own limits, credentials, environment, directory.
    In interactor mode, however, -i and -o have no effects on both main
    program and interactor.
  -ri=<f>       - in interactor mode, record program input to file <f>.
  -ro=<f>       - in interactor mode, record program output to file <f>.
  -interaction-log=<f> - in interactor mode, record interaction to file <f>

Process properties:
  -t <value>    - time limit. Terminate after <value> seconds, you can use
                  suffix ms to switch to milliseconds. Suffix "s" (seconds)
                  can be omitted.
  -h <value>	- wall time limit. Terminate after <value> real time seconds,
  				  you can use suffix ms to switch to milliseconds.
				  Suffix "s" (seconds) can be omitted
  -m <value>    - memory limit. Terminate if anonymous virtual memory of the
                  process exceeds <value> bytes. Use suffixes K, M, G to
                  specify kilo, mega, gigabytes.
  -D k=v        - environment. If any is specified, existing environment is
				  cleared.
  -envfile <filename> - if specified, the file is loaded as new process environment.
  -d <value>    - current directory for the process.
  -l <value>    - login name. Create process under <value> user.
  -p <value>    - password for user specified in -l. On linux, ignored (but
                  must be present).
  -j <filename> - inject <filename> DLL into process.
  -i <filename> - redirect standard input to <filename>.
  -o <filename> - redirect standard output to <filename>.
  -e <filename> - redirect standard error to <filename>.
  -os <value>   - limit size of standard output file to <value>.
  -es <value>   - limit size of standard error file to <value>.
  -u            - instead of using separate stderr, join error output to standard output.
  -no-idleness-check - switch off idleness checking.
  -a <value>	- set process affinity to <value>. You can either specify it
                  as plain int, or as a bit mask starting with 0, so 2 and
                  010 are equivalent.

  Some options require job objects to function. When process is created, runexe attempts
  to create a job object. If it can't, it will continue without it unless internal flag
  FailOnJobCreationFailure is set (which is false by default). If job object can be created, then:
    - extra set of UI restrictions is applied unless -z is specified
	- flag to force close on unhandled exception is set
	- hard time limit of "time limit + 1s" is applied

  Options which control job object behavior:
  -no-job       - don't even try creating job objects.
  -z            - don't restrict process UI. This option doesn't set
				  FailOnJobCreationFailure, so if job creation fails the job will run
				  unrestricted.
  -process-limit <intvalue> - set process limit to <intvalue>. Windows will refuse
                  to create processes above the limit. Setting this sets FailOnJobCreationFailure
				  and is incompatible with -no-job.
`
