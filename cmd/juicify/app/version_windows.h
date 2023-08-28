/******************************************************************************
 *
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 *
 *****************************************************************************/
#include <Windows.h>

typedef const char* (*GetVersionFn)();

const char * GetJuiceVersion(const char* library, int *err)
{
    HMODULE handle = LoadLibraryA(library);
    if (handle != NULL)
    {
        const char* version = NULL;

        GetVersionFn getVersion = (GetVersionFn)GetProcAddress(handle, "GetVersion");
        if (getVersion != NULL)
        {
            version = getVersion();
            *err = 0;
        }
        else
        {
            *err = GetLastError();
        }

        FreeLibrary(handle);

        return version;
    }

    *err = GetLastError();
    return NULL;
}
