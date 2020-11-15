//
//桥接lua和golang的c代码
#include "_obj/_cgo_export.h"
#include "xlua.h"

static void go_lua_hook_func(lua_State *L, lua_Debug *ar) {
    LuaOnStep(G(L)->ud);
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

static void *l_alloc(void *ud, void *ptr, size_t osize, size_t nsize) {
	(void)ud; (void)osize;  /* not used */
	if (nsize == 0) {
		free(ptr);
		return NULL;
	}
	return realloc(ptr, nsize);
}

static void warnf(void *ud, const char *message, int tocont) {
	int *warnstate = (int *)ud;
	if (*warnstate != 2 && !tocont && *message == '@') {  /* control message? */
		if (strcmp(message, "@off") == 0)
			*warnstate = 0;
		else if (strcmp(message, "@on") == 0)
			*warnstate = 1;
		return;
	}
	else if (*warnstate == 0)  /* warnings off? */
		return;
	if (*warnstate == 1)  /* previous message was the last? */
		lua_writestringerror("%s", "Lua warning: ");  /* start a new warning */
	lua_writestringerror("%s", message);  /* write message */
	if (tocont)  /* not the last part? */
		*warnstate = 2;  /* to be continued */
	else {  /* last part */
		lua_writestringerror("%s", "\n");  /* finish message with end-of-line */
		*warnstate = 1;  /* ready to start a new message */
	}
}

lua_State *go_lua_newstate(void *s) {
	lua_State *L = lua_newstate(l_alloc, s);
	if (L) {
		int *warnstate;  /* space for warning state */
		lua_atpanic(L, &go_lua_at_panic);
		warnstate = (int *)lua_newuserdatauv(L, sizeof(int), 0);
		luaL_ref(L, LUA_REGISTRYINDEX);  /* make sure it won't be collected */
		*warnstate = 0;  /* default is warnings off */
		lua_setwarnf(L, warnf, warnstate);
		lua_sethook(L, go_lua_hook_func, LUA_MASKCOUNT, 1);
	}
	return L;
}

static int callgofunc(lua_State *L) {
    void *f = lua_touserdata(L,lua_upvalueindex(1));
    return (int)CallGoFunc(G(L)->ud,f);
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