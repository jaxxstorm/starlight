// Native GL.iNet portal component. Requests use the portal's authenticated RPC.
module.exports = {
  name: "StarlightView",
  data: function () {
    return { status: null, error: "", notice: "", busy: false, host: "", port: 8444, psk: "", initialized: false, benchmark: null, benchError: "", benchmarking: false };
  },
  created: function () { this.refresh(); },
  methods: {
    refresh: function () {
      var self = this;
      return window.$request("call", ["sid", "starlight", "get_status", {}]).then(function (result) {
        if (!result || result.err_msg) throw new Error(result && result.err_msg || "Unable to read service status");
        self.status = result;
        if (!self.initialized) { self.host = result.host; self.port = result.port; self.initialized = true; }
        self.error = "";
      }).catch(function (err) { self.error = err.message || "Unable to read service status"; });
    },
    setEnabled: function (enabled) {
      var self = this;
      self.busy = true;
      window.$request("call", ["sid", "starlight", "set_enabled", { enabled: enabled }]).then(function (result) {
        if (!result || result.err_msg) throw new Error(result && result.err_msg || "Unable to change service state");
        self.status = result;
        self.error = "";
      }).catch(function (err) { self.error = err.message || "Unable to change service state"; })
        .then(function () { self.busy = false; });
    },
    setRouterWide: function (enabled) {
      var self = this;
      self.busy = true;
      window.$request("call", ["sid", "starlight", "set_router_wide", { enabled: enabled }]).then(function (result) {
        if (!result || result.err_msg) throw new Error(result && result.err_msg || "Unable to change router-wide routing");
        self.status = result;
        self.error = "";
        setTimeout(function () { self.refresh(); }, 5000);
      }).catch(function (err) { self.error = err.message || "Unable to change router-wide routing"; })
        .then(function () { self.busy = false; });
    },
    saveSettings: function () {
      var self = this, params = { host: self.host.trim(), port: Number(self.port) };
      if (self.psk) params.psk = self.psk.trim();
      self.busy = true; self.error = ""; self.notice = "";
      window.$request("call", ["sid", "starlight", "set_config", params]).then(function (result) {
        if (!result || result.err_msg) throw new Error(result && result.err_msg || "Could not save settings");
        self.status = result; self.psk = ""; self.notice = "Settings saved. Test connectivity to confirm the server uses the same PSK.";
        setTimeout(function () { self.refresh(); }, 5000);
      }).catch(function (err) { self.error = err.message || "Could not save settings"; })
        .then(function () { self.busy = false; });
    },
    testConnectivity: function () {
      var self = this;
      self.busy = true; self.error = ""; self.notice = "Testing saved settings…";
      window.$request("call", ["sid", "starlight", "test_connectivity", {}], { timeout: 15000 }).then(function (result) {
        if (!result || !result.success) throw new Error("Connection or authentication failed. Check the saved address, port, PSK and ground server.");
        self.notice = "Connected and authenticated successfully using the saved settings.";
      }).catch(function (err) { self.notice = ""; self.error = err.message || "Connectivity test failed"; })
        .then(function () { self.busy = false; });
    },
    runBenchmark: function () {
      var self = this;
      self.benchmarking = true; self.benchError = ""; self.benchmark = null;
      window.$request("call", ["sid", "starlight", "run_benchmark", {}], { timeout: 40000 }).then(function (result) {
        if (!result || result.err_msg || !Array.isArray(result.rows)) throw new Error(result && result.err_msg || "Benchmark failed");
        self.benchmark = result;
      }).catch(function (err) { self.benchError = err.message || "Benchmark failed"; })
        .then(function () { self.benchmarking = false; });
    },
    generateKey: function () {
      var self = this;
      self.busy = true; self.error = ""; self.notice = "";
      window.$request("call", ["sid", "starlight", "generate_psk", {}]).then(function (result) {
        if (!result || !result.psk) throw new Error("Could not generate a PSK");
        self.psk = result.psk;
        var blob = new Blob([result.psk + "\n"], { type: "text/plain" });
        var url = URL.createObjectURL(blob), link = document.createElement("a");
        link.href = url; link.download = "starlight.psk"; document.body.appendChild(link); link.click(); link.remove();
        setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
        self.notice = "PSK generated and download requested. Configure the ground server with the same PSK, then save settings here.";
      }).catch(function (err) { self.error = err.message || "Could not generate a PSK"; })
        .then(function () { self.busy = false; });
    }
  },
  render: function (h) {
    var self = this;
    var row = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: "24px", padding: "18px 0" };
    var card = { padding: "24px", marginTop: "20px", maxWidth: "900px", background: "#fff", borderRadius: "8px" };
    return h("div", { style: { padding: "24px" } }, [
      h("gl-title", { props: { title: "Starlight" }, attrs: { title: "Starlight" } }),
      h("div", { staticClass: "is-general-card", style: card }, [
        h("div", { style: row }, [
          h("div", [
            h("strong", "Starlight client"),
            h("p", { style: { marginTop: "8px", color: "#667085" } }, self.status ? (self.status.running ? "Running" : "Stopped") : "Checking status…")
          ]),
          h("gl-switch", {
            props: { value: !!(self.status && self.status.enabled), disabled: self.busy || !self.status, size: "small" },
            on: { input: function (value) { self.setEnabled(value); } }
          })
        ]),
        h("div", { style: Object.assign({}, row, { borderTop: "1px solid #e5e7eb" }) }, [
          h("div", [
            h("strong", "Router-wide traffic"),
            h("p", { style: { marginTop: "8px", color: "#667085" } }, self.status && self.status.router_wide_enabled ? (self.status.router_wide_active ? "IPv4 TCP capture active" : "Enabled; waiting for healthy connection or falling back to normal routing") : "Disabled. IPv4 TCP can use Starlight; DNS, UDP and IPv6 use normal routing.")
          ]),
          h("gl-switch", { props: { value: !!(self.status && self.status.router_wide_enabled), disabled: self.busy || !self.status, size: "small" }, on: { input: function (value) { self.setRouterWide(value); } } })
        ]),
        self.error ? h("p", { style: { color: "#b42318" } }, self.error) : null
      ]),
      h("div", { staticClass: "is-general-card", style: card }, [
        h("h3", "Ground server"),
        h("p", "When router-wide mode is enabled, use an IPv4 address. Save settings before testing."),
        h("div", { style: row }, [h("label", "Server address"), h("el-input", { props: { value: self.host, placeholder: "143.198.79.144" }, on: { input: function (v) { self.host = v; } }, style: { maxWidth: "320px" } })]),
        h("div", { style: row }, [h("label", "Port"), h("el-input", { props: { value: String(self.port), placeholder: "18444" }, on: { input: function (v) { self.port = v; } }, style: { maxWidth: "320px" } })]),
        h("div", { style: row }, [h("label", "New PSK (hex; leave blank to keep current)"), h("el-input", { props: { value: self.psk, type: "password", placeholder: "Leave blank to keep current PSK" }, on: { input: function (v) { self.psk = v; } }, style: { maxWidth: "320px" } })]),
        h("p", "Changing the PSK disconnects the client until the ground server uses the same key. Send the downloaded key over a secure channel."),
        h("div", { style: { display: "flex", gap: "12px", flexWrap: "wrap" } }, [
          h("gl-button", { attrs: { type: "primary", disabled: self.busy }, on: { click: self.saveSettings } }, "Save settings"),
          h("gl-button", { attrs: { disabled: self.busy }, on: { click: self.testConnectivity } }, "Test connectivity"),
          h("gl-button", { attrs: { disabled: self.busy }, on: { click: self.generateKey } }, "Generate and download PSK")
        ]),
        self.notice ? h("p", { style: { marginTop: "16px" } }, self.notice) : null,
        self.error ? h("p", { style: { color: "#b42318", marginTop: "16px" } }, self.error) : null
      ]),
      h("div", { staticClass: "is-general-card", style: card }, [
        h("h3", "Benchmark"),
        h("p", "Compare direct router requests with Starlight ground fetch. Each run contacts six public sites twice and may use several MiB of data. Results are from the router, not individual clients."),
        self.status && !self.status.running ? h("p", { style: { color: "#b54708" } }, "The Starlight client is stopped; ground-fetch requests will fail until you enable it.") : null,
        h("gl-button", { attrs: { type: "primary", disabled: self.benchmarking }, on: { click: self.runBenchmark } }, self.benchmarking ? "Running benchmark…" : "Run benchmark"),
        self.benchError ? h("p", { style: { color: "#b42318", marginTop: "16px" } }, self.benchError) : null,
        self.benchmark ? h("div", { style: { marginTop: "20px", overflowX: "auto" } }, [
          h("p", "Burst time — direct: " + (self.benchmark.direct_wall_ms / 1000).toFixed(2) + "s (" + self.benchmark.rows.filter(function (r) { return !r.direct.ok; }).length + " failed); Starlight: " + (self.benchmark.mux_wall_ms / 1000).toFixed(2) + "s (" + self.benchmark.rows.filter(function (r) { return !r.mux.ok; }).length + " failed). Burst times including failures are not speed comparisons."),
          h("table", { style: { width: "100%", textAlign: "left", borderCollapse: "collapse" } }, [
            h("thead", [h("tr", ["Origin", "Direct", "Starlight", "Speedup"].map(function (text) { return h("th", { style: { padding: "10px", borderBottom: "1px solid #ddd" } }, text); }))]),
            h("tbody", self.benchmark.rows.map(function (r) {
              var fmt = function (m) { return m.ok ? (m.total_ms / 1000).toFixed(2) + "s (TTFB " + (m.ttfb_ms / 1000).toFixed(2) + "s)" : (m.error || "Failed"); };
              var ratio = r.direct.ok && r.mux.ok && r.mux.total_ms > 0 ? (r.direct.total_ms / r.mux.total_ms).toFixed(1) + "×" : "—";
              return h("tr", [r.name, fmt(r.direct), fmt(r.mux), ratio].map(function (text) { return h("td", { style: { padding: "10px", borderBottom: "1px solid #eee" } }, text); }));
            }))
          ])
        ]) : null
      ])
    ]);
  }
};
