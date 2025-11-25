#include <iostream>

using namespace std;

int main()
{
	while (true)
	{
		string s;
		cin >> s;
		if (s == "!")
		{
			break;
		}
		cout << s << ' ';
		fflush(stdout);
	}
	return 0;
}
