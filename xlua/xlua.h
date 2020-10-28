
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include "lapi.h"
#include "lualib.h"
#include "lauxlib.h"

void go_lua_state_init(lua_State *L,void *s);

void go_lua_set_func(lua_State *L,const char *name,void *f);