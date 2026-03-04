@echo off
call "C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Auxiliary\Build\vcvarsall.bat" x64 >nul 2>&1
cd /d C:\Users\stingray\Documents\godev\m\runlib\rust
echo LIB_IS=%LIB%
echo INCLUDE_IS=%INCLUDE%
where link.exe
cargo build 2>&1
echo EXIT_CODE=%ERRORLEVEL%
