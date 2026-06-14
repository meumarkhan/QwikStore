package commands

import (
	"fmt"
	"runtime"
	"strings"
	"time"
	"qwikstore/resp"
)

var startTime = time.Now()

func cmdPing(ctx *Context) *resp.Value {
	if ctx.NArgs() == 0 {
		return &resp.Value{Type: resp.TypeSimpleString, Str: "PONG"}
	}
	return bulkResp(ctx.ArgStr(0))
}

func cmdEcho(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("ECHO")
	}
	return bulkResp(ctx.ArgStr(0))
}

// cmdSelect is handled specially in the server (needs to change active DB index).
// Here we just return OK as a placeholder; the server intercepts it.
func cmdSelect(ctx *Context) *resp.Value {
	return okResp()
}

func cmdDBSize(ctx *Context) *resp.Value {
	return intResp(int64(ctx.DB.Size()))
}

func cmdFlushDB(ctx *Context) *resp.Value {
	ctx.DB.Flush()
	return okResp()
}

func cmdFlushAll(ctx *Context) *resp.Value {
	// Server-level flush is handled by the executor using the store reference.
	// For now flush the current DB.
	ctx.DB.Flush()
	return okResp()
}

func cmdInfo(ctx *Context) *resp.Value {
	section := "all"
	if ctx.NArgs() >= 1 {
		section = strings.ToLower(ctx.ArgStr(0))
	}

	var sb strings.Builder

	if section == "all" || section == "server" {
		sb.WriteString("# Server\r\n")
		sb.WriteString("redis_version:7.0.0-qwikstore\r\n")
		sb.WriteString("os:" + runtime.GOOS + "\r\n")
		sb.WriteString("arch_bits:64\r\n")
		sb.WriteString(fmt.Sprintf("uptime_in_seconds:%d\r\n", int64(time.Since(startTime).Seconds())))
		sb.WriteString(fmt.Sprintf("uptime_in_days:%d\r\n", int64(time.Since(startTime).Hours()/24)))
		sb.WriteString("hz:10\r\n")
		sb.WriteString("\r\n")
	}
	if section == "all" || section == "clients" {
		sb.WriteString("# Clients\r\n")
		sb.WriteString("connected_clients:1\r\n")
		sb.WriteString("\r\n")
	}
	if section == "all" || section == "memory" {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		sb.WriteString("# Memory\r\n")
		sb.WriteString(fmt.Sprintf("used_memory:%d\r\n", ms.Alloc))
		sb.WriteString(fmt.Sprintf("used_memory_human:%dK\r\n", ms.Alloc/1024))
		sb.WriteString(fmt.Sprintf("used_memory_rss:%d\r\n", ms.Sys))
		sb.WriteString("\r\n")
	}
	if section == "all" || section == "stats" {
		sb.WriteString("# Stats\r\n")
		sb.WriteString("total_connections_received:0\r\n")
		sb.WriteString("total_commands_processed:0\r\n")
		sb.WriteString("\r\n")
	}
	if section == "all" || section == "keyspace" {
		sb.WriteString("# Keyspace\r\n")
		sb.WriteString(fmt.Sprintf("db0:keys=%d\r\n", ctx.DB.Size()))
		sb.WriteString("\r\n")
	}

	return bulkResp(sb.String())
}

func cmdConfig(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("CONFIG")
	}
	sub := strings.ToUpper(ctx.ArgStr(0))
	switch sub {
	case "GET":
		if ctx.NArgs() < 2 {
			return wrongArgCount("CONFIG GET")
		}
		// Return empty for unknown params
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	case "SET":
		if ctx.NArgs() < 3 {
			return wrongArgCount("CONFIG SET")
		}
		return okResp()
	case "RESETSTAT":
		return okResp()
	case "REWRITE":
		return okResp()
	}
	return errResp("ERR unknown subcommand '" + ctx.ArgStr(0) + "'")
}

func cmdCommand(ctx *Context) *resp.Value {
	if ctx.NArgs() == 0 {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	sub := strings.ToUpper(ctx.ArgStr(0))
	switch sub {
	case "COUNT":
		return intResp(0)
	case "DOCS":
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	case "INFO":
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
}

func cmdDebug(ctx *Context) *resp.Value {
	if ctx.NArgs() >= 1 && strings.ToUpper(ctx.ArgStr(0)) == "SLEEP" {
		if ctx.NArgs() >= 2 {
			var secs float64
			fmt.Sscanf(ctx.ArgStr(1), "%f", &secs)
			time.Sleep(time.Duration(secs * float64(time.Second)))
		}
		return okResp()
	}
	return okResp()
}

func cmdSave(ctx *Context) *resp.Value {
	// AOF already handles persistence; SAVE is a no-op here
	return okResp()
}

func cmdBgSave(ctx *Context) *resp.Value {
	return &resp.Value{Type: resp.TypeSimpleString, Str: "Background saving started"}
}

func cmdLastSave(ctx *Context) *resp.Value {
	return intResp(startTime.Unix())
}
