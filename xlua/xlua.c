//
//桥接lua和golang的c代码
#include "_obj/_cgo_export.h"
#include "xlua.h"

static void go_lua_hook_func(lua_State *L, lua_Debug *ar) {
    LuaOnStep(L->goptr);
}

static GoString togostring(const char *v)  {
    GoString str = {.p=v,.n=strlen(v)};
    return str;
}

static int go_lua_at_panic(lua_State *L) {
    size_t len = 0;
    const char *msg = lua_tolstring(L, 1, &len);
    Panic(togostring(msg));
    return 0;
}

void go_lua_state_init(lua_State *L) {
    lua_sethook(L, go_lua_hook_func, LUA_MASKCOUNT, 1);
    lua_atpanic(L, go_lua_at_panic);
}

static int callgofunc(lua_State *L) {
    void *f = lua_touserdata(L,lua_upvalueindex(1));
    return (int)CallGoFunc(L->goptr,f);
}

void go_lua_set_global_func(lua_State *L,const char *name,void *f) {
    go_lua_push_func(L,f);
    lua_setglobal(L, name);
}

void go_lua_push_func(lua_State *L,void *f) {
    lua_pushlightuserdata(L,f);
    lua_pushcclosure(L,callgofunc,1);
}


bool go_lua_is_array(lua_State *L, int *len,int idx) {
	if (!lua_istable(L, idx)) {
		return false;
	}
	bool isarr = true;
	lua_Integer idxv = 1;
	lua_pushnil(L);
	while (lua_next(L,idx-1) != LUA_TNIL) {
		int kt = lua_type(L, -2);
		if (kt != LUA_TNUMBER) {
			isarr = false;
			lua_pop(L, 2);
			break;
		}
		int isnum = 0;
		int kv = (int)lua_tointegerx(L, -2, &isnum);
		if (!isnum) {
			isarr = false;
			lua_pop(L, 2);
			break;
		}
		if (kv != idxv) {
			isarr = false;
			lua_pop(L, 2);
			break;
		}
		idxv++;
		lua_pop(L, 1);
	}
	if (len) {
		*len = idxv - 1;
	}
	return isarr;
}