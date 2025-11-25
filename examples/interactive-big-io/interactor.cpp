#include "testlib.h"
#include <iostream>

using namespace std;

int main(int argc, char *argv[])
{
	setName("Interactor.");
	registerInteraction(argc, argv);

	int number = inf.readInt();
	for (int i = 0; i < number; i++)
	{
		cout << i << endl
			 << i << endl;
		fflush(stdout);
		ouf.readToken();
		ouf.readToken();
	}
	cout << "!" << endl;
	fflush(stdout);
	quitf(_ok, "The end.");
}