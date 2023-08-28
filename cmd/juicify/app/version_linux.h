/******************************************************************************
 *
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 *
 *****************************************************************************/
#include <dlfcn.h>
#include <stdlib.h>

typedef const char* (*GetVersionFn)();

const char * GetJuiceVersion(const char* library, char **err)
{
    void* handle = dlopen(library, RTLD_NOW);
    if (handle != NULL)
    {
        const char* version = NULL;

        GetVersionFn getVersion = (GetVersionFn)dlsym(handle, "GetVersion");
        if (getVersion != NULL)
        {
            version = getVersion();
            *err = NULL;
        }
        else
        {
            *err = dlerror();
        }

        dlclose(handle);

        return version;
    }

    *err = dlerror();
    return NULL;
}
