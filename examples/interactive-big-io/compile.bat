set home=%GCC6_HOME%
if not "%home%"=="" set home=%home%\bin\
set PATH=%home%;%PATH%

set CXX=%home%g++ -Wall -Wextra -Wconversion -static -DONLINE_JUDGE -Wl,--stack=268435456 -O2 -std=c++14

%CXX% -o interactor.exe interactor.cpp
if %errorlevel% neq 0 exit
%CXX% -o solution.exe solution.cpp
if %errorlevel% neq 0 exit