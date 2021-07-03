#pragma runtime_checks("", off)

#pragma comment(linker, "/nodefaultlib /subsystem:console /ENTRY:test")

#define WIN32_LEAN_AND_MEAN
#include <windows.h>
//#include <strsafe.h>

#define BUFSIZE 64

void reverse(char str[], int length)
{
    int start = 0;
    int end = length -1;
    while (start < end)
    {
	char t = *(str+start);
	*(str+start) = *(str+end);
	*(str+end) = t;
        start++;
        end--;
    }
}
  
// Implementation of itoa()
int itoa(int num, char* str, int base)
{
    int i = 0;
    int isNegative = 0;
  
    /* Handle 0 explicitely, otherwise empty string is printed for 0 */
    if (num == 0)
    {
        str[i++] = '0';
        str[i] = '\0';
        return 1;
    }
  
    // In standard itoa(), negative numbers are handled only with 
    // base 10. Otherwise numbers are considered unsigned.
    if (num < 0 && base == 10)
    {
        isNegative = 1;
        num = -num;
    }
  
    // Process individual digits
    while (num != 0)
    {
        int rem = num % base;
        str[i++] = (rem > 9)? (rem-10) + 'a' : rem + '0';
        num = num/base;
    }
  
    // If number is negative, append '-'
    if (isNegative)
        str[i++] = '-';
  
    str[i] = '\0'; // Append string terminator
  
    // Reverse the string
    reverse(str, i);
  
    return i;
}


int __stdcall test(void)
{
    DWORD written = 0;
    char pszDest[BUFSIZE];
    int chars = itoa((int)LoadLibraryW, pszDest, 10);
    pszDest[chars+1] = 0;
    pszDest[chars] = '\n';

    HANDLE stdOut = GetStdHandle(STD_OUTPUT_HANDLE);
    WriteFile(stdOut, pszDest, chars, &written, NULL);
    ExitProcess(0);
}

/*
int main() {
    TCHAR pszDest[BUFSIZE];

    HRESULT hr = StringCchPrintf(pszDest, BUFSIZE, TEXT("%d\n"), (int)LoadLibraryA);

    if (FAILED(hr)) { ExitProcess(1); };

    size_t actual_length = 0;
    hr = StringCchLength(pszDest, BUFSIZE, &actual_length);
    if (FAILED(hr)) { ExitProcess(1); };

    DWORD written = 0;

    HANDLE stdOut = GetStdHandle(STD_OUTPUT_HANDLE);
    WriteFile(stdOut, pszDest, actual_length, &written, NULL);
    ExitProcess(0);
} */