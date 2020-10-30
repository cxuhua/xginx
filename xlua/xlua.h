
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <stdbool.h>
#include "lapi.h"
#include "lualib.h"
#include "lauxlib.h"

void go_lua_state_init(lua_State *L);

void go_lua_set_global_func(lua_State *L,const char *name,void *f);

void go_lua_push_func(lua_State *L,void *f);

bool go_lua_is_array(lua_State *L,int *len,int idx);
