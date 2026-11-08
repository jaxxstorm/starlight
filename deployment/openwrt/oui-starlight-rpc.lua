-- Native GL.iNet portal RPC. No unauthenticated methods are registered.
local rpc = require "oui.rpc"
local M = {}

local function authorized()
	return rpc.access("starlight", "service", "w")
end

local function command_ok(command)
	return os.execute(command .. " >/dev/null 2>&1") == 0
end

local config_path = "/etc/starlight/client.conf"
local function read_config()
	local f = io.open(config_path, "r")
	if not f then return nil end
	local text = f:read("*a")
	f:close()
	return text
end

local function config_value(config, name)
	return ("\n" .. config):match("\n" .. name .. "=([^\n]*)")
end

local function save_config(config, values)
	local lines = {}
	for line in (config .. "\n"):gmatch("([^\n]*)\n") do
		local key = line:match("^([%w_]+)=")
		if line ~= "" and not (key and values[key] ~= nil) then lines[#lines + 1] = line end
	end
	for key, value in pairs(values) do lines[#lines + 1] = key .. "=" .. value end
	local temp = config_path .. ".tmp"
	local f = io.open(temp, "w")
	if not f then return false end
	local ok = f:write(table.concat(lines, "\n") .. "\n")
	local closed = f:close()
	if not ok or not closed or not command_ok("chmod 600 /etc/starlight/client.conf.tmp") or not os.rename(temp, config_path) then
		os.remove(temp)
		return false
	end
	return true
end

local function restart_if_running()
	return not command_ok("/etc/init.d/starlight running") or command_ok("/usr/libexec/starlight-restart")
end

local function valid_ipv4(host)
	local a, b, c, d = host:match("^(%d+)%.(%d+)%.(%d+)%.(%d+)$")
	if not a then return false end
	for _, octet in ipairs({a, b, c, d}) do
		if #octet > 3 or tonumber(octet) > 255 then return false end
	end
	return true
end

function M.get_status()
	if not authorized() then return rpc.ERROR_CODE_ACCESS end
	local config = read_config() or ""
	return {
		running = command_ok("/etc/init.d/starlight running"),
		enabled = command_ok("/etc/init.d/starlight enabled"),
		router_wide_available = true,
		router_wide_enabled = config:match("\nrouter_wide=1\n") ~= nil,
		router_wide_active = command_ok("/usr/sbin/nft list set inet starlight_router capture_lease | grep -q br-lan")
		, host = config_value(config, "host") or "", port = tonumber(config_value(config, "port")) or 0
	}
end

function M.set_config(params)
	if not authorized() then return rpc.ERROR_CODE_ACCESS end
	if type(params) ~= "table" or type(params.host) ~= "string" or #params.host < 1 or #params.host > 253 or not params.host:match("^[%w%.:%-]+$") or type(params.port) ~= "number" or params.port % 1 ~= 0 or params.port < 1 or params.port > 65535 then return rpc.ERROR_CODE_INVALID_PARAMS end
	if params.psk ~= nil and (type(params.psk) ~= "string" or #params.psk < 2 or #params.psk > 256 or #params.psk % 2 ~= 0 or not params.psk:match("^[%da-fA-F]+$")) then return rpc.ERROR_CODE_INVALID_PARAMS end
	local config = read_config()
	if not config then return { err_code = -1, err_msg = "Missing client configuration" } end
	if config_value(config, "router_wide") == "1" and not valid_ipv4(params.host) then
		return { err_code = -1, err_msg = "Router-wide mode requires an IPv4 ground address" }
	end
	local values = { host = params.host, port = tostring(params.port) }
	if params.psk then values.psk = params.psk end
	if not save_config(config, values) then return { err_code = -1, err_msg = "Cannot save configuration" } end
	if not restart_if_running() then return { err_code = -1, err_msg = "Configuration saved but restart failed" } end
	return M.get_status()
end

function M.generate_psk()
	if not authorized() then return rpc.ERROR_CODE_ACCESS end
	local f = io.open("/dev/urandom", "rb")
	if not f then return { err_code = -1, err_msg = "Random generator unavailable" } end
	local bytes = f:read(32)
	f:close()
	if not bytes or #bytes ~= 32 then return { err_code = -1, err_msg = "Random generator failed" } end
	return { psk = (bytes:gsub(".", function(c) return string.format("%02x", c:byte()) end)) }
end

function M.test_connectivity()
	if not authorized() then return rpc.ERROR_CODE_ACCESS end
	return { success = command_ok("/usr/libexec/starlight-probe") }
end

function M.run_benchmark()
	if not authorized() then return rpc.ERROR_CODE_ACCESS end
	local config = read_config()
	if not config then return { err_code = -1, err_msg = "Missing client configuration" } end
	local port = config_value(config, "fast_port") or "8081"
	if not port:match("^%d+$") or not tonumber(port) or tonumber(port) < 1 or tonumber(port) > 65535 then
		return { err_code = -1, err_msg = "Invalid fast-fetch port" }
	end
	-- Only a validated numeric port is interpolated; URLs are fixed in the Go binary.
	local p = io.popen("flock -n /var/lock/starlight-bench.lock /usr/sbin/starlight bench --fast-port " .. port .. " 2>/dev/null", "r")
	if not p then return { err_code = -1, err_msg = "Could not start benchmark" } end
	local output = p:read("*a")
	p:close()
	local ok, result = pcall(require("cjson").decode, output)
	if not ok or type(result) ~= "table" or type(result.rows) ~= "table" then
		return { err_code = -1, err_msg = "Benchmark unavailable or already running" }
	end
	return result
end

function M.set_router_wide(params)
	if not authorized() then return rpc.ERROR_CODE_ACCESS end
	if type(params) ~= "table" or type(params.enabled) ~= "boolean" then return rpc.ERROR_CODE_INVALID_PARAMS end
	local config = read_config()
	if not config then return { err_code = -1, err_msg = "Missing client configuration" } end
	local lines = {}
	for line in (config .. "\n"):gmatch("([^\n]*)\n") do
		if line ~= "" and not line:match("^router_wide=") and not line:match("^transparent_port=") then
			lines[#lines + 1] = line
		end
	end
	lines[#lines + 1] = "transparent_port=" .. (params.enabled and "19090" or "0")
	lines[#lines + 1] = "router_wide=" .. (params.enabled and "1" or "0")
	local temp = config_path .. ".tmp"
	local f = io.open(temp, "w")
	if not f then return { err_code = -1, err_msg = "Cannot update configuration" } end
	local ok = f:write(table.concat(lines, "\n") .. "\n")
	local closed = f:close()
	if not ok or not closed or not command_ok("chmod 600 /etc/starlight/client.conf.tmp") or not os.rename(temp, config_path) then
		os.remove(temp)
		return { err_code = -1, err_msg = "Cannot save configuration" }
	end
	if not restart_if_running() then
		return { err_code = -1, err_msg = "Configuration saved but client restart failed" }
	end
	return M.get_status()
end

function M.set_enabled(params)
	if not authorized() then return rpc.ERROR_CODE_ACCESS end
	if type(params) ~= "table" or type(params.enabled) ~= "boolean" then
		return rpc.ERROR_CODE_INVALID_PARAMS
	end
	local ok
	if params.enabled then
		ok = command_ok("/etc/init.d/starlight enable && /etc/init.d/starlight start")
	else
		ok = command_ok("/etc/init.d/starlight stop && /etc/init.d/starlight disable")
	end
	if not ok then return { err_code = -1, err_msg = "Unable to change Starlight service state" } end
	return M.get_status()
end

return M
