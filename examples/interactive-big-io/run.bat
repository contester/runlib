set RUNEXE=..\..\runexe.exe

    @REM -ilog=log.txt ^
%RUNEXE% -xml -a 8 -t 1000ms -h 36000ms -m 16777216 ^
    -os 67108864 -es 67108864 ^
    --interactor="-a 16 -d . -t 60000ms -h 240000ms -m 536870912 -no-idleness-check -process-limit 1 interactor.exe input.txt output.txt" ^
    -process-limit 1 solution.exe


@REM INFO  [2025-11-24 18:23:34,701] InvokeRunner: Running '"C:\Users\kuviman\Work\.data\invoker\work\41b494cdef41047db80efab02034f6fc\check-fb7efd927aafa3109448a3b3f93f6a6d\run\runexe2.exe" -xml -a 8 -t 6000ms -h 36000ms -m 16777216 -i input.fd0138e687.txt -o output.fd0138e687.txt -e invocation-standard-error.tmp -envfile b98adc.env -os 67108864 -es 67108864 --interactor="-a 16 -d C:\Users\kuviman\Work\.data\invoker\work\41b494cdef41047db80efab02034f6fc\check-fb7efd927aafa3109448a3b3f93f6a6d\interactor -e C:\Users\kuviman\Work\.data\invoker\work\41b494cdef41047db80efab02034f6fc\check-fb7efd927aafa3109448a3b3f93f6a6d\interactor\invocation-standard-error.tmp -t 60000ms -h 240000ms -m 536870912 -no-idleness-check -process-limit 1 C:\Users\kuviman\Work\.data\invoker\work\41b494cdef41047db80efab02034f6fc\check-fb7efd927aafa3109448a3b3f93f6a6d\interactor\interactor.exe C:\Users\kuviman\Work\.data\invoker\work\41b494cdef41047db80efab02034f6fc\check-fb7efd927aafa3109448a3b3f93f6a6d\interactor\input.fd0138e687.txt C:\Users\kuviman\Work\.data\invoker\work\41b494cdef41047db80efab02034f6fc\check-fb7efd927aafa3109448a3b3f93f6a6d\interactor\output.fd0138e687.txt"  -process-limit 1 25807e9cdaabf1e1d01d537ad7966e51.exe', directory='C:\Users\kuviman\Work\.data\invoker\work\41b494cdef41047db80efab02034f6fc\check-fb7efd927aafa3109448a3b3f93f6a6d\run'.