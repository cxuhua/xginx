//


#include "_obj/_cgo_export.h"
#include "xlua.h"

static void go_lua_hook_func(lua_State *L, lua_Debug *ar) {
    LuaOnStep(L->goptr);
}

static int go_lua_at_panic(lua_State *L) {
    size_t len = 0;
    const char *msg = lua_tolstring(L, 1, &len);
    GoString str = {.p=msg,.n=len};
    Panic(str);
    return 0;
}

void go_lua_state_init(lua_State *L,void *s) {
    L->goptr = s;
    lua_sethook(L, go_lua_hook_func, LUA_MASKCOUNT, 1);
    lua_atpanic(L, go_lua_at_panic);
}

static int callgofunc(lua_State *L) {
    void *f = lua_touserdata(L,lua_upvalueindex(1));
    CallGoFunc(L->goptr,f);
}

void go_lua_set_func(lua_State *L,const char *name,void *f) {
    lua_pushlightuserdata(L,f);
    lua_pushcclosure(L,callgofunc,1);
    lua_setglobal(L, name);
}