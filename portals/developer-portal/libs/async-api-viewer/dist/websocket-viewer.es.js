import * as b from "react";
import ht, { isValidElement as $a, cloneElement as Ra, Children as Id, lazy as Ad, Suspense as fi, forwardRef as Rc, useContext as Md, useState as jt, createElement as Nd, useEffect as pi, useRef as Ld, useMemo as Tc } from "react";
import * as Ut from "react-dom";
import ga from "react-dom";
var va = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : typeof self < "u" ? self : {};
function Tr(t) {
  return t && t.__esModule && Object.prototype.hasOwnProperty.call(t, "default") ? t.default : t;
}
var Ro = { exports: {} }, tn = {};
/*
object-assign
(c) Sindre Sorhus
@license MIT
*/
var To, Ns;
function Dd() {
  if (Ns) return To;
  Ns = 1;
  var t = Object.getOwnPropertySymbols, e = Object.prototype.hasOwnProperty, a = Object.prototype.propertyIsEnumerable;
  function r(o) {
    if (o == null)
      throw new TypeError("Object.assign cannot be called with null or undefined");
    return Object(o);
  }
  function n() {
    try {
      if (!Object.assign)
        return !1;
      var o = new String("abc");
      if (o[5] = "de", Object.getOwnPropertyNames(o)[0] === "5")
        return !1;
      for (var i = {}, s = 0; s < 10; s++)
        i["_" + String.fromCharCode(s)] = s;
      var l = Object.getOwnPropertyNames(i).map(function(c) {
        return i[c];
      });
      if (l.join("") !== "0123456789")
        return !1;
      var u = {};
      return "abcdefghijklmnopqrst".split("").forEach(function(c) {
        u[c] = c;
      }), Object.keys(Object.assign({}, u)).join("") === "abcdefghijklmnopqrst";
    } catch {
      return !1;
    }
  }
  return To = n() ? Object.assign : function(o, i) {
    for (var s, l = r(o), u, c = 1; c < arguments.length; c++) {
      s = Object(arguments[c]);
      for (var d in s)
        e.call(s, d) && (l[d] = s[d]);
      if (t) {
        u = t(s);
        for (var p = 0; p < u.length; p++)
          a.call(s, u[p]) && (l[u[p]] = s[u[p]]);
      }
    }
    return l;
  }, To;
}
/** @license React v17.0.2
 * react-jsx-runtime.production.min.js
 *
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var Ls;
function Bd() {
  if (Ls) return tn;
  Ls = 1, Dd();
  var t = ht, e = 60103;
  if (tn.Fragment = 60107, typeof Symbol == "function" && Symbol.for) {
    var a = Symbol.for;
    e = a("react.element"), tn.Fragment = a("react.fragment");
  }
  var r = t.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED.ReactCurrentOwner, n = Object.prototype.hasOwnProperty, o = { key: !0, ref: !0, __self: !0, __source: !0 };
  function i(s, l, u) {
    var c, d = {}, p = null, h = null;
    u !== void 0 && (p = "" + u), l.key !== void 0 && (p = "" + l.key), l.ref !== void 0 && (h = l.ref);
    for (c in l) n.call(l, c) && !o.hasOwnProperty(c) && (d[c] = l[c]);
    if (s && s.defaultProps) for (c in l = s.defaultProps, l) d[c] === void 0 && (d[c] = l[c]);
    return { $$typeof: e, type: s, key: p, ref: h, props: d, _owner: r.current };
  }
  return tn.jsx = i, tn.jsxs = i, tn;
}
var Ds;
function Fd() {
  return Ds || (Ds = 1, Ro.exports = Bd()), Ro.exports;
}
var B = Fd(), Ba = {
  black: "#000",
  white: "#fff"
}, Oo = {
  300: "#e57373",
  500: "#f44336",
  700: "#d32f2f"
}, _o = {
  A200: "#ff4081",
  A400: "#f50057",
  A700: "#c51162"
}, Po = {
  300: "#7986cb",
  500: "#3f51b5",
  700: "#303f9f"
}, jo = {
  300: "#64b5f6",
  500: "#2196f3",
  700: "#1976d2"
}, Io = {
  300: "#81c784",
  500: "#4caf50",
  700: "#388e3c"
}, Ao = {
  300: "#ffb74d",
  500: "#ff9800",
  700: "#f57c00"
}, Hi = {
  50: "#fafafa",
  100: "#f5f5f5",
  200: "#eeeeee",
  300: "#e0e0e0",
  400: "#bdbdbd",
  500: "#9e9e9e",
  600: "#757575",
  700: "#616161",
  800: "#424242",
  900: "#212121",
  A100: "#d5d5d5",
  A200: "#aaaaaa",
  A400: "#303030",
  A700: "#616161"
};
function Y() {
  return Y = Object.assign ? Object.assign.bind() : function(t) {
    for (var e = 1; e < arguments.length; e++) {
      var a = arguments[e];
      for (var r in a) ({}).hasOwnProperty.call(a, r) && (t[r] = a[r]);
    }
    return t;
  }, Y.apply(null, arguments);
}
function Sr(t) {
  "@babel/helpers - typeof";
  return Sr = typeof Symbol == "function" && typeof Symbol.iterator == "symbol" ? function(e) {
    return typeof e;
  } : function(e) {
    return e && typeof Symbol == "function" && e.constructor === Symbol && e !== Symbol.prototype ? "symbol" : typeof e;
  }, Sr(t);
}
function Mo(t) {
  return t && Sr(t) === "object" && t.constructor === Object;
}
function wr(t, e) {
  var a = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : {
    clone: !0
  }, r = a.clone ? Y({}, t) : t;
  return Mo(t) && Mo(e) && Object.keys(e).forEach(function(n) {
    n !== "__proto__" && (Mo(e[n]) && n in t ? r[n] = wr(t[n], e[n], a) : r[n] = e[n]);
  }), r;
}
function zd(t, e) {
  if (Sr(t) != "object" || !t) return t;
  var a = t[Symbol.toPrimitive];
  if (a !== void 0) {
    var r = a.call(t, e);
    if (Sr(r) != "object") return r;
    throw new TypeError("@@toPrimitive must return a primitive value.");
  }
  return (e === "string" ? String : Number)(t);
}
function Oc(t) {
  var e = zd(t, "string");
  return Sr(e) == "symbol" ? e : e + "";
}
function Ot(t, e, a) {
  return (e = Oc(e)) in t ? Object.defineProperty(t, e, {
    value: a,
    enumerable: !0,
    configurable: !0,
    writable: !0
  }) : t[e] = a, t;
}
function vn(t) {
  for (var e = "https://mui.com/production-error/?code=" + t, a = 1; a < arguments.length; a += 1)
    e += "&args[]=" + encodeURIComponent(arguments[a]);
  return "Minified Material-UI error #" + t + "; visit " + e + " for the full message.";
}
var No = { exports: {} }, gt = {};
/** @license React v17.0.2
 * react-is.production.min.js
 *
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var Bs;
function Wd() {
  if (Bs) return gt;
  Bs = 1;
  var t = 60103, e = 60106, a = 60107, r = 60108, n = 60114, o = 60109, i = 60110, s = 60112, l = 60113, u = 60120, c = 60115, d = 60116, p = 60121, h = 60122, g = 60117, m = 60129, y = 60131;
  if (typeof Symbol == "function" && Symbol.for) {
    var w = Symbol.for;
    t = w("react.element"), e = w("react.portal"), a = w("react.fragment"), r = w("react.strict_mode"), n = w("react.profiler"), o = w("react.provider"), i = w("react.context"), s = w("react.forward_ref"), l = w("react.suspense"), u = w("react.suspense_list"), c = w("react.memo"), d = w("react.lazy"), p = w("react.block"), h = w("react.server.block"), g = w("react.fundamental"), m = w("react.debug_trace_mode"), y = w("react.legacy_hidden");
  }
  function k(C) {
    if (typeof C == "object" && C !== null) {
      var M = C.$$typeof;
      switch (M) {
        case t:
          switch (C = C.type, C) {
            case a:
            case n:
            case r:
            case l:
            case u:
              return C;
            default:
              switch (C = C && C.$$typeof, C) {
                case i:
                case s:
                case d:
                case c:
                case o:
                  return C;
                default:
                  return M;
              }
          }
        case e:
          return M;
      }
    }
  }
  var j = o, f = t, x = s, E = a, L = d, W = c, O = e, P = n, A = r, v = l;
  return gt.ContextConsumer = i, gt.ContextProvider = j, gt.Element = f, gt.ForwardRef = x, gt.Fragment = E, gt.Lazy = L, gt.Memo = W, gt.Portal = O, gt.Profiler = P, gt.StrictMode = A, gt.Suspense = v, gt.isAsyncMode = function() {
    return !1;
  }, gt.isConcurrentMode = function() {
    return !1;
  }, gt.isContextConsumer = function(C) {
    return k(C) === i;
  }, gt.isContextProvider = function(C) {
    return k(C) === o;
  }, gt.isElement = function(C) {
    return typeof C == "object" && C !== null && C.$$typeof === t;
  }, gt.isForwardRef = function(C) {
    return k(C) === s;
  }, gt.isFragment = function(C) {
    return k(C) === a;
  }, gt.isLazy = function(C) {
    return k(C) === d;
  }, gt.isMemo = function(C) {
    return k(C) === c;
  }, gt.isPortal = function(C) {
    return k(C) === e;
  }, gt.isProfiler = function(C) {
    return k(C) === n;
  }, gt.isStrictMode = function(C) {
    return k(C) === r;
  }, gt.isSuspense = function(C) {
    return k(C) === l;
  }, gt.isValidElementType = function(C) {
    return typeof C == "string" || typeof C == "function" || C === a || C === n || C === m || C === r || C === l || C === u || C === y || typeof C == "object" && C !== null && (C.$$typeof === d || C.$$typeof === c || C.$$typeof === o || C.$$typeof === i || C.$$typeof === s || C.$$typeof === g || C.$$typeof === p || C[0] === h);
  }, gt.typeOf = k, gt;
}
var Fs;
function Vd() {
  return Fs || (Fs = 1, No.exports = Wd()), No.exports;
}
Vd();
function Ki(t) {
  var e = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : 0, a = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : 1;
  return Math.min(Math.max(e, t), a);
}
function Hd(t) {
  t = t.substr(1);
  var e = new RegExp(".{1,".concat(t.length >= 6 ? 2 : 1, "}"), "g"), a = t.match(e);
  return a && a[0].length === 1 && (a = a.map(function(r) {
    return r + r;
  })), a ? "rgb".concat(a.length === 4 ? "a" : "", "(").concat(a.map(function(r, n) {
    return n < 3 ? parseInt(r, 16) : Math.round(parseInt(r, 16) / 255 * 1e3) / 1e3;
  }).join(", "), ")") : "";
}
function Kd(t) {
  t = Vr(t);
  var e = t, a = e.values, r = a[0], n = a[1] / 100, o = a[2] / 100, i = n * Math.min(o, 1 - o), s = function(d) {
    var p = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : (d + r / 30) % 12;
    return o - i * Math.max(Math.min(p - 3, 9 - p, 1), -1);
  }, l = "rgb", u = [Math.round(s(0) * 255), Math.round(s(8) * 255), Math.round(s(4) * 255)];
  return t.type === "hsla" && (l += "a", u.push(a[3])), Ga({
    type: l,
    values: u
  });
}
function Vr(t) {
  if (t.type)
    return t;
  if (t.charAt(0) === "#")
    return Vr(Hd(t));
  var e = t.indexOf("("), a = t.substring(0, e);
  if (["rgb", "rgba", "hsl", "hsla"].indexOf(a) === -1)
    throw new Error(vn(3, t));
  var r = t.substring(e + 1, t.length - 1).split(",");
  return r = r.map(function(n) {
    return parseFloat(n);
  }), {
    type: a,
    values: r
  };
}
function Ga(t) {
  var e = t.type, a = t.values;
  return e.indexOf("rgb") !== -1 ? a = a.map(function(r, n) {
    return n < 3 ? parseInt(r, 10) : r;
  }) : e.indexOf("hsl") !== -1 && (a[1] = "".concat(a[1], "%"), a[2] = "".concat(a[2], "%")), "".concat(e, "(").concat(a.join(", "), ")");
}
function Ud(t, e) {
  var a = hi(t), r = hi(e);
  return (Math.max(a, r) + 0.05) / (Math.min(a, r) + 0.05);
}
function hi(t) {
  t = Vr(t);
  var e = t.type === "hsl" ? Vr(Kd(t)).values : t.values;
  return e = e.map(function(a) {
    return a /= 255, a <= 0.03928 ? a / 12.92 : Math.pow((a + 0.055) / 1.055, 2.4);
  }), Number((0.2126 * e[0] + 0.7152 * e[1] + 0.0722 * e[2]).toFixed(3));
}
function rn(t) {
  var e = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : 0.15;
  return hi(t) > 0.5 ? _c(t, e) : Pc(t, e);
}
function Ye(t, e) {
  return t = Vr(t), e = Ki(e), (t.type === "rgb" || t.type === "hsl") && (t.type += "a"), t.values[3] = e, Ga(t);
}
function _c(t, e) {
  if (t = Vr(t), e = Ki(e), t.type.indexOf("hsl") !== -1)
    t.values[2] *= 1 - e;
  else if (t.type.indexOf("rgb") !== -1)
    for (var a = 0; a < 3; a += 1)
      t.values[a] *= 1 - e;
  return Ga(t);
}
function Pc(t, e) {
  if (t = Vr(t), e = Ki(e), t.type.indexOf("hsl") !== -1)
    t.values[2] += (100 - t.values[2]) * e;
  else if (t.type.indexOf("rgb") !== -1)
    for (var a = 0; a < 3; a += 1)
      t.values[a] += (255 - t.values[a]) * e;
  return Ga(t);
}
function Ya(t, e) {
  if (t == null) return {};
  var a = {};
  for (var r in t) if ({}.hasOwnProperty.call(t, r)) {
    if (e.indexOf(r) !== -1) continue;
    a[r] = t[r];
  }
  return a;
}
function Ae(t, e) {
  if (t == null) return {};
  var a, r, n = Ya(t, e);
  if (Object.getOwnPropertySymbols) {
    var o = Object.getOwnPropertySymbols(t);
    for (r = 0; r < o.length; r++) a = o[r], e.indexOf(a) === -1 && {}.propertyIsEnumerable.call(t, a) && (n[a] = t[a]);
  }
  return n;
}
var gr = ["xs", "sm", "md", "lg", "xl"];
function qd(t) {
  var e = t.values, a = e === void 0 ? {
    xs: 0,
    sm: 600,
    md: 960,
    lg: 1280,
    xl: 1920
  } : e, r = t.unit, n = r === void 0 ? "px" : r, o = t.step, i = o === void 0 ? 5 : o, s = Ae(t, ["values", "unit", "step"]);
  function l(h) {
    var g = typeof a[h] == "number" ? a[h] : h;
    return "@media (min-width:".concat(g).concat(n, ")");
  }
  function u(h) {
    var g = gr.indexOf(h) + 1, m = a[gr[g]];
    if (g === gr.length)
      return l("xs");
    var y = typeof m == "number" && g > 0 ? m : h;
    return "@media (max-width:".concat(y - i / 100).concat(n, ")");
  }
  function c(h, g) {
    var m = gr.indexOf(g);
    return m === gr.length - 1 ? l(h) : "@media (min-width:".concat(typeof a[h] == "number" ? a[h] : h).concat(n, ") and ") + "(max-width:".concat((m !== -1 && typeof a[gr[m + 1]] == "number" ? a[gr[m + 1]] : g) - i / 100).concat(n, ")");
  }
  function d(h) {
    return c(h, h);
  }
  function p(h) {
    return a[h];
  }
  return Y({
    keys: gr,
    values: a,
    up: l,
    down: u,
    between: c,
    only: d,
    width: p
  }, s);
}
function Gd(t, e, a) {
  var r;
  return Y({
    gutters: function() {
      var o = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : {};
      return console.warn(["Material-UI: theme.mixins.gutters() is deprecated.", "You can use the source of the mixin directly:", `
      paddingLeft: theme.spacing(2),
      paddingRight: theme.spacing(2),
      [theme.breakpoints.up('sm')]: {
        paddingLeft: theme.spacing(3),
        paddingRight: theme.spacing(3),
      },
      `].join(`
`)), Y({
        paddingLeft: e(2),
        paddingRight: e(2)
      }, o, Ot({}, t.up("sm"), Y({
        paddingLeft: e(3),
        paddingRight: e(3)
      }, o[t.up("sm")])));
    },
    toolbar: (r = {
      minHeight: 56
    }, Ot(r, "".concat(t.up("xs"), " and (orientation: landscape)"), {
      minHeight: 48
    }), Ot(r, t.up("sm"), {
      minHeight: 64
    }), r)
  }, a);
}
var zs = {
  // The colors used to style the text.
  text: {
    // The most important text.
    primary: "rgba(0, 0, 0, 0.87)",
    // Secondary text.
    secondary: "rgba(0, 0, 0, 0.54)",
    // Disabled text have even lower visual prominence.
    disabled: "rgba(0, 0, 0, 0.38)",
    // Text hints.
    hint: "rgba(0, 0, 0, 0.38)"
  },
  // The color used to divide different elements.
  divider: "rgba(0, 0, 0, 0.12)",
  // The background colors used to style the surfaces.
  // Consistency between these values is important.
  background: {
    paper: Ba.white,
    default: Hi[50]
  },
  // The colors used to style the action elements.
  action: {
    // The color of an active action like an icon button.
    active: "rgba(0, 0, 0, 0.54)",
    // The color of an hovered action.
    hover: "rgba(0, 0, 0, 0.04)",
    hoverOpacity: 0.04,
    // The color of a selected action.
    selected: "rgba(0, 0, 0, 0.08)",
    selectedOpacity: 0.08,
    // The color of a disabled action.
    disabled: "rgba(0, 0, 0, 0.26)",
    // The background color of a disabled action.
    disabledBackground: "rgba(0, 0, 0, 0.12)",
    disabledOpacity: 0.38,
    focus: "rgba(0, 0, 0, 0.12)",
    focusOpacity: 0.12,
    activatedOpacity: 0.12
  }
}, Lo = {
  text: {
    primary: Ba.white,
    secondary: "rgba(255, 255, 255, 0.7)",
    disabled: "rgba(255, 255, 255, 0.5)",
    hint: "rgba(255, 255, 255, 0.5)",
    icon: "rgba(255, 255, 255, 0.5)"
  },
  divider: "rgba(255, 255, 255, 0.12)",
  background: {
    paper: Hi[800],
    default: "#303030"
  },
  action: {
    active: Ba.white,
    hover: "rgba(255, 255, 255, 0.08)",
    hoverOpacity: 0.08,
    selected: "rgba(255, 255, 255, 0.16)",
    selectedOpacity: 0.16,
    disabled: "rgba(255, 255, 255, 0.3)",
    disabledBackground: "rgba(255, 255, 255, 0.12)",
    disabledOpacity: 0.38,
    focus: "rgba(255, 255, 255, 0.12)",
    focusOpacity: 0.12,
    activatedOpacity: 0.24
  }
};
function Ws(t, e, a, r) {
  var n = r.light || r, o = r.dark || r * 1.5;
  t[e] || (t.hasOwnProperty(a) ? t[e] = t[a] : e === "light" ? t.light = Pc(t.main, n) : e === "dark" && (t.dark = _c(t.main, o)));
}
function Yd(t) {
  var e = t.primary, a = e === void 0 ? {
    light: Po[300],
    main: Po[500],
    dark: Po[700]
  } : e, r = t.secondary, n = r === void 0 ? {
    light: _o.A200,
    main: _o.A400,
    dark: _o.A700
  } : r, o = t.error, i = o === void 0 ? {
    light: Oo[300],
    main: Oo[500],
    dark: Oo[700]
  } : o, s = t.warning, l = s === void 0 ? {
    light: Ao[300],
    main: Ao[500],
    dark: Ao[700]
  } : s, u = t.info, c = u === void 0 ? {
    light: jo[300],
    main: jo[500],
    dark: jo[700]
  } : u, d = t.success, p = d === void 0 ? {
    light: Io[300],
    main: Io[500],
    dark: Io[700]
  } : d, h = t.type, g = h === void 0 ? "light" : h, m = t.contrastThreshold, y = m === void 0 ? 3 : m, w = t.tonalOffset, k = w === void 0 ? 0.2 : w, j = Ae(t, ["primary", "secondary", "error", "warning", "info", "success", "type", "contrastThreshold", "tonalOffset"]);
  function f(W) {
    var O = Ud(W, Lo.text.primary) >= y ? Lo.text.primary : zs.text.primary;
    return O;
  }
  var x = function(O) {
    var P = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : 500, A = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : 300, v = arguments.length > 3 && arguments[3] !== void 0 ? arguments[3] : 700;
    if (O = Y({}, O), !O.main && O[P] && (O.main = O[P]), !O.main)
      throw new Error(vn(4, P));
    if (typeof O.main != "string")
      throw new Error(vn(5, JSON.stringify(O.main)));
    return Ws(O, "light", A, k), Ws(O, "dark", v, k), O.contrastText || (O.contrastText = f(O.main)), O;
  }, E = {
    dark: Lo,
    light: zs
  }, L = wr(Y({
    // A collection of common colors.
    common: Ba,
    // The palette type, can be light or dark.
    type: g,
    // The colors used to represent primary interface elements for a user.
    primary: x(a),
    // The colors used to represent secondary interface elements for a user.
    secondary: x(n, "A400", "A200", "A700"),
    // The colors used to represent interface elements that the user should be made aware of.
    error: x(i),
    // The colors used to represent potentially dangerous actions or important messages.
    warning: x(l),
    // The colors used to present information to the user that is neutral and not necessarily important.
    info: x(c),
    // The colors used to indicate the successful completion of an action that user triggered.
    success: x(p),
    // The grey colors.
    grey: Hi,
    // Used by `getContrastText()` to maximize the contrast between
    // the background and the text.
    contrastThreshold: y,
    // Takes a background color and returns the text color that maximizes the contrast.
    getContrastText: f,
    // Generate a rich color object.
    augmentColor: x,
    // Used by the functions below to shift a color's luminance by approximately
    // two indexes within its tonal palette.
    // E.g., shift from Red 500 to Red 300 or Red 700.
    tonalOffset: k
  }, E[g]), j);
  return L;
}
function jc(t) {
  return Math.round(t * 1e5) / 1e5;
}
function Xd(t) {
  return jc(t);
}
var Vs = {
  textTransform: "uppercase"
}, Hs = '"Roboto", "Helvetica", "Arial", sans-serif';
function Jd(t, e) {
  var a = typeof e == "function" ? e(t) : e, r = a.fontFamily, n = r === void 0 ? Hs : r, o = a.fontSize, i = o === void 0 ? 14 : o, s = a.fontWeightLight, l = s === void 0 ? 300 : s, u = a.fontWeightRegular, c = u === void 0 ? 400 : u, d = a.fontWeightMedium, p = d === void 0 ? 500 : d, h = a.fontWeightBold, g = h === void 0 ? 700 : h, m = a.htmlFontSize, y = m === void 0 ? 16 : m, w = a.allVariants, k = a.pxToRem, j = Ae(a, ["fontFamily", "fontSize", "fontWeightLight", "fontWeightRegular", "fontWeightMedium", "fontWeightBold", "htmlFontSize", "allVariants", "pxToRem"]), f = i / 14, x = k || function(W) {
    return "".concat(W / y * f, "rem");
  }, E = function(O, P, A, v, C) {
    return Y({
      fontFamily: n,
      fontWeight: O,
      fontSize: x(P),
      // Unitless following https://meyerweb.com/eric/thoughts/2006/02/08/unitless-line-heights/
      lineHeight: A
    }, n === Hs ? {
      letterSpacing: "".concat(jc(v / P), "em")
    } : {}, C, w);
  }, L = {
    h1: E(l, 96, 1.167, -1.5),
    h2: E(l, 60, 1.2, -0.5),
    h3: E(c, 48, 1.167, 0),
    h4: E(c, 34, 1.235, 0.25),
    h5: E(c, 24, 1.334, 0),
    h6: E(p, 20, 1.6, 0.15),
    subtitle1: E(c, 16, 1.75, 0.15),
    subtitle2: E(p, 14, 1.57, 0.1),
    body1: E(c, 16, 1.5, 0.15),
    body2: E(c, 14, 1.43, 0.15),
    button: E(p, 14, 1.75, 0.4, Vs),
    caption: E(c, 12, 1.66, 0.4),
    overline: E(c, 12, 2.66, 1, Vs)
  };
  return wr(Y({
    htmlFontSize: y,
    pxToRem: x,
    round: Xd,
    // TODO v5: remove
    fontFamily: n,
    fontSize: i,
    fontWeightLight: l,
    fontWeightRegular: c,
    fontWeightMedium: p,
    fontWeightBold: g
  }, L), j, {
    clone: !1
    // No need to clone deep
  });
}
var Zd = 0.2, Qd = 0.14, ef = 0.12;
function xt() {
  return ["".concat(arguments.length <= 0 ? void 0 : arguments[0], "px ").concat(arguments.length <= 1 ? void 0 : arguments[1], "px ").concat(arguments.length <= 2 ? void 0 : arguments[2], "px ").concat(arguments.length <= 3 ? void 0 : arguments[3], "px rgba(0,0,0,").concat(Zd, ")"), "".concat(arguments.length <= 4 ? void 0 : arguments[4], "px ").concat(arguments.length <= 5 ? void 0 : arguments[5], "px ").concat(arguments.length <= 6 ? void 0 : arguments[6], "px ").concat(arguments.length <= 7 ? void 0 : arguments[7], "px rgba(0,0,0,").concat(Qd, ")"), "".concat(arguments.length <= 8 ? void 0 : arguments[8], "px ").concat(arguments.length <= 9 ? void 0 : arguments[9], "px ").concat(arguments.length <= 10 ? void 0 : arguments[10], "px ").concat(arguments.length <= 11 ? void 0 : arguments[11], "px rgba(0,0,0,").concat(ef, ")")].join(",");
}
var tf = ["none", xt(0, 2, 1, -1, 0, 1, 1, 0, 0, 1, 3, 0), xt(0, 3, 1, -2, 0, 2, 2, 0, 0, 1, 5, 0), xt(0, 3, 3, -2, 0, 3, 4, 0, 0, 1, 8, 0), xt(0, 2, 4, -1, 0, 4, 5, 0, 0, 1, 10, 0), xt(0, 3, 5, -1, 0, 5, 8, 0, 0, 1, 14, 0), xt(0, 3, 5, -1, 0, 6, 10, 0, 0, 1, 18, 0), xt(0, 4, 5, -2, 0, 7, 10, 1, 0, 2, 16, 1), xt(0, 5, 5, -3, 0, 8, 10, 1, 0, 3, 14, 2), xt(0, 5, 6, -3, 0, 9, 12, 1, 0, 3, 16, 2), xt(0, 6, 6, -3, 0, 10, 14, 1, 0, 4, 18, 3), xt(0, 6, 7, -4, 0, 11, 15, 1, 0, 4, 20, 3), xt(0, 7, 8, -4, 0, 12, 17, 2, 0, 5, 22, 4), xt(0, 7, 8, -4, 0, 13, 19, 2, 0, 5, 24, 4), xt(0, 7, 9, -4, 0, 14, 21, 2, 0, 5, 26, 4), xt(0, 8, 9, -5, 0, 15, 22, 2, 0, 6, 28, 5), xt(0, 8, 10, -5, 0, 16, 24, 2, 0, 6, 30, 5), xt(0, 8, 11, -5, 0, 17, 26, 2, 0, 6, 32, 5), xt(0, 9, 11, -5, 0, 18, 28, 2, 0, 7, 34, 6), xt(0, 9, 12, -6, 0, 19, 29, 2, 0, 7, 36, 6), xt(0, 10, 13, -6, 0, 20, 31, 3, 0, 8, 38, 7), xt(0, 10, 13, -6, 0, 21, 33, 3, 0, 8, 40, 7), xt(0, 10, 14, -6, 0, 22, 35, 3, 0, 8, 42, 7), xt(0, 11, 14, -7, 0, 23, 36, 3, 0, 9, 44, 8), xt(0, 11, 15, -7, 0, 24, 38, 3, 0, 9, 46, 8)], rf = {
  borderRadius: 4
};
function mi(t, e) {
  (e == null || e > t.length) && (e = t.length);
  for (var a = 0, r = Array(e); a < e; a++) r[a] = t[a];
  return r;
}
function nf(t) {
  if (Array.isArray(t)) return mi(t);
}
function Ic(t) {
  if (typeof Symbol < "u" && t[Symbol.iterator] != null || t["@@iterator"] != null) return Array.from(t);
}
function Ui(t, e) {
  if (t) {
    if (typeof t == "string") return mi(t, e);
    var a = {}.toString.call(t).slice(8, -1);
    return a === "Object" && t.constructor && (a = t.constructor.name), a === "Map" || a === "Set" ? Array.from(t) : a === "Arguments" || /^(?:Ui|I)nt(?:8|16|32)(?:Clamped)?Array$/.test(a) ? mi(t, e) : void 0;
  }
}
function af() {
  throw new TypeError(`Invalid attempt to spread non-iterable instance.
In order to be iterable, non-array objects must have a [Symbol.iterator]() method.`);
}
function Xa(t) {
  return nf(t) || Ic(t) || Ui(t) || af();
}
function Fa(t, e) {
  return e ? wr(t, e, {
    clone: !1
    // No need to clone deep, it's way faster.
  }) : t;
}
var of = {
  xs: 0,
  sm: 600,
  md: 960,
  lg: 1280,
  xl: 1920
}, Ks = {
  // Sorted ASC by size. That's important.
  // It can't be configured as it's used statically for propTypes.
  keys: ["xs", "sm", "md", "lg", "xl"],
  up: function(e) {
    return "@media (min-width:".concat(of[e], "px)");
  }
};
function Ac(t, e, a) {
  if (Array.isArray(e)) {
    var r = t.theme.breakpoints || Ks;
    return e.reduce(function(i, s, l) {
      return i[r.up(r.keys[l])] = a(e[l]), i;
    }, {});
  }
  if (Sr(e) === "object") {
    var n = t.theme.breakpoints || Ks;
    return Object.keys(e).reduce(function(i, s) {
      return i[n.up(s)] = a(e[s]), i;
    }, {});
  }
  var o = a(e);
  return o;
}
function Us(t, e) {
  return !e || typeof e != "string" ? null : e.split(".").reduce(function(a, r) {
    return a && a[r] ? a[r] : null;
  }, t);
}
function Re(t) {
  var e = t.prop, a = t.cssProperty, r = a === void 0 ? t.prop : a, n = t.themeKey, o = t.transform, i = function(l) {
    if (l[e] == null)
      return null;
    var u = l[e], c = l.theme, d = Us(c, n) || {}, p = function(g) {
      var m;
      return typeof d == "function" ? m = d(g) : Array.isArray(d) ? m = d[g] || g : (m = Us(d, g) || g, o && (m = o(m))), r === !1 ? m : Ot({}, r, m);
    };
    return Ac(l, u, p);
  };
  return i.propTypes = {}, i.filterProps = [e], i;
}
function dr() {
  for (var t = arguments.length, e = new Array(t), a = 0; a < t; a++)
    e[a] = arguments[a];
  var r = function(o) {
    return e.reduce(function(i, s) {
      var l = s(o);
      return l ? Fa(i, l) : i;
    }, {});
  };
  return r.propTypes = {}, r.filterProps = e.reduce(function(n, o) {
    return n.concat(o.filterProps);
  }, []), r;
}
function ta(t) {
  return typeof t != "number" ? t : "".concat(t, "px solid");
}
var sf = Re({
  prop: "border",
  themeKey: "borders",
  transform: ta
}), lf = Re({
  prop: "borderTop",
  themeKey: "borders",
  transform: ta
}), cf = Re({
  prop: "borderRight",
  themeKey: "borders",
  transform: ta
}), uf = Re({
  prop: "borderBottom",
  themeKey: "borders",
  transform: ta
}), df = Re({
  prop: "borderLeft",
  themeKey: "borders",
  transform: ta
}), ff = Re({
  prop: "borderColor",
  themeKey: "palette"
}), pf = Re({
  prop: "borderRadius",
  themeKey: "shape"
}), hf = dr(sf, lf, cf, uf, df, ff, pf);
function qs(t, e) {
  var a = {};
  return Object.keys(t).forEach(function(r) {
    e.indexOf(r) === -1 && (a[r] = t[r]);
  }), a;
}
function mf(t) {
  var e = function(r) {
    var n = t(r);
    return r.css ? Y({}, Fa(n, t(Y({
      theme: r.theme
    }, r.css))), qs(r.css, [t.filterProps])) : r.sx ? Y({}, Fa(n, t(Y({
      theme: r.theme
    }, r.sx))), qs(r.sx, [t.filterProps])) : n;
  };
  return e.propTypes = {}, e.filterProps = ["css", "sx"].concat(Xa(t.filterProps)), e;
}
var gf = Re({
  prop: "displayPrint",
  cssProperty: !1,
  transform: function(e) {
    return {
      "@media print": {
        display: e
      }
    };
  }
}), vf = Re({
  prop: "display"
}), bf = Re({
  prop: "overflow"
}), yf = Re({
  prop: "textOverflow"
}), xf = Re({
  prop: "visibility"
}), Cf = Re({
  prop: "whiteSpace"
});
const Sf = dr(gf, vf, bf, yf, xf, Cf);
var wf = Re({
  prop: "flexBasis"
}), Ef = Re({
  prop: "flexDirection"
}), kf = Re({
  prop: "flexWrap"
}), $f = Re({
  prop: "justifyContent"
}), Rf = Re({
  prop: "alignItems"
}), Tf = Re({
  prop: "alignContent"
}), Of = Re({
  prop: "order"
}), _f = Re({
  prop: "flex"
}), Pf = Re({
  prop: "flexGrow"
}), jf = Re({
  prop: "flexShrink"
}), If = Re({
  prop: "alignSelf"
}), Af = Re({
  prop: "justifyItems"
}), Mf = Re({
  prop: "justifySelf"
}), Nf = dr(wf, Ef, kf, $f, Rf, Tf, Of, _f, Pf, jf, If, Af, Mf), Lf = Re({
  prop: "gridGap"
}), Df = Re({
  prop: "gridColumnGap"
}), Bf = Re({
  prop: "gridRowGap"
}), Ff = Re({
  prop: "gridColumn"
}), zf = Re({
  prop: "gridRow"
}), Wf = Re({
  prop: "gridAutoFlow"
}), Vf = Re({
  prop: "gridAutoColumns"
}), Hf = Re({
  prop: "gridAutoRows"
}), Kf = Re({
  prop: "gridTemplateColumns"
}), Uf = Re({
  prop: "gridTemplateRows"
}), qf = Re({
  prop: "gridTemplateAreas"
}), Gf = Re({
  prop: "gridArea"
}), Yf = dr(Lf, Df, Bf, Ff, zf, Wf, Vf, Hf, Kf, Uf, qf, Gf), Xf = Re({
  prop: "color",
  themeKey: "palette"
}), Jf = Re({
  prop: "bgcolor",
  cssProperty: "backgroundColor",
  themeKey: "palette"
}), Zf = dr(Xf, Jf), Qf = Re({
  prop: "position"
}), ep = Re({
  prop: "zIndex",
  themeKey: "zIndex"
}), tp = Re({
  prop: "top"
}), rp = Re({
  prop: "right"
}), np = Re({
  prop: "bottom"
}), ap = Re({
  prop: "left"
});
const op = dr(Qf, ep, tp, rp, np, ap);
var ip = Re({
  prop: "boxShadow",
  themeKey: "shadows"
});
function Or(t) {
  return t <= 1 ? "".concat(t * 100, "%") : t;
}
var sp = Re({
  prop: "width",
  transform: Or
}), lp = Re({
  prop: "maxWidth",
  transform: Or
}), cp = Re({
  prop: "minWidth",
  transform: Or
}), up = Re({
  prop: "height",
  transform: Or
}), dp = Re({
  prop: "maxHeight",
  transform: Or
}), fp = Re({
  prop: "minHeight",
  transform: Or
});
Re({
  prop: "size",
  cssProperty: "width",
  transform: Or
});
Re({
  prop: "size",
  cssProperty: "height",
  transform: Or
});
var pp = Re({
  prop: "boxSizing"
}), hp = dr(sp, lp, cp, up, dp, fp, pp);
function Mc(t) {
  if (Array.isArray(t)) return t;
}
function mp(t, e) {
  var a = t == null ? null : typeof Symbol < "u" && t[Symbol.iterator] || t["@@iterator"];
  if (a != null) {
    var r, n, o, i, s = [], l = !0, u = !1;
    try {
      if (o = (a = a.call(t)).next, e !== 0) for (; !(l = (r = o.call(a)).done) && (s.push(r.value), s.length !== e); l = !0) ;
    } catch (c) {
      u = !0, n = c;
    } finally {
      try {
        if (!l && a.return != null && (i = a.return(), Object(i) !== i)) return;
      } finally {
        if (u) throw n;
      }
    }
    return s;
  }
}
function Nc() {
  throw new TypeError(`Invalid attempt to destructure non-iterable instance.
In order to be iterable, non-array objects must have a [Symbol.iterator]() method.`);
}
function En(t, e) {
  return Mc(t) || mp(t, e) || Ui(t, e) || Nc();
}
function gp(t) {
  var e = {};
  return function(a) {
    return e[a] === void 0 && (e[a] = t(a)), e[a];
  };
}
var vp = {
  m: "margin",
  p: "padding"
}, bp = {
  t: "Top",
  r: "Right",
  b: "Bottom",
  l: "Left",
  x: ["Left", "Right"],
  y: ["Top", "Bottom"]
}, Gs = {
  marginX: "mx",
  marginY: "my",
  paddingX: "px",
  paddingY: "py"
}, yp = gp(function(t) {
  if (t.length > 2)
    if (Gs[t])
      t = Gs[t];
    else
      return [t];
  var e = t.split(""), a = En(e, 2), r = a[0], n = a[1], o = vp[r], i = bp[n] || "";
  return Array.isArray(i) ? i.map(function(s) {
    return o + s;
  }) : [o + i];
}), Lc = ["m", "mt", "mr", "mb", "ml", "mx", "my", "p", "pt", "pr", "pb", "pl", "px", "py", "margin", "marginTop", "marginRight", "marginBottom", "marginLeft", "marginX", "marginY", "padding", "paddingTop", "paddingRight", "paddingBottom", "paddingLeft", "paddingX", "paddingY"];
function Dc(t) {
  var e = t.spacing || 8;
  return typeof e == "number" ? function(a) {
    return e * a;
  } : Array.isArray(e) ? function(a) {
    return e[a];
  } : typeof e == "function" ? e : function() {
  };
}
function xp(t, e) {
  if (typeof e == "string" || e == null)
    return e;
  var a = Math.abs(e), r = t(a);
  return e >= 0 ? r : typeof r == "number" ? -r : "-".concat(r);
}
function Cp(t, e) {
  return function(a) {
    return t.reduce(function(r, n) {
      return r[n] = xp(e, a), r;
    }, {});
  };
}
function qi(t) {
  var e = t.theme, a = Dc(e);
  return Object.keys(t).map(function(r) {
    if (Lc.indexOf(r) === -1)
      return null;
    var n = yp(r), o = Cp(n, a), i = t[r];
    return Ac(t, i, o);
  }).reduce(Fa, {});
}
qi.propTypes = {};
qi.filterProps = Lc;
var Sp = Re({
  prop: "fontFamily",
  themeKey: "typography"
}), wp = Re({
  prop: "fontSize",
  themeKey: "typography"
}), Ep = Re({
  prop: "fontStyle",
  themeKey: "typography"
}), kp = Re({
  prop: "fontWeight",
  themeKey: "typography"
}), $p = Re({
  prop: "letterSpacing"
}), Rp = Re({
  prop: "lineHeight"
}), Tp = Re({
  prop: "textAlign"
}), Op = dr(Sp, wp, Ep, kp, $p, Rp, Tp);
function _p() {
  var t = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : 8;
  if (t.mui)
    return t;
  var e = Dc({
    spacing: t
  }), a = function() {
    for (var n = arguments.length, o = new Array(n), i = 0; i < n; i++)
      o[i] = arguments[i];
    return o.length === 0 ? e(1) : o.length === 1 ? e(o[0]) : o.map(function(s) {
      if (typeof s == "string")
        return s;
      var l = e(s);
      return typeof l == "number" ? "".concat(l, "px") : l;
    }).join(" ");
  };
  return Object.defineProperty(a, "unit", {
    get: function() {
      return t;
    }
  }), a.mui = !0, a;
}
var Ys = {
  // This is the most common easing curve.
  easeInOut: "cubic-bezier(0.4, 0, 0.2, 1)",
  // Objects enter the screen at full velocity from off-screen and
  // slowly decelerate to a resting point.
  easeOut: "cubic-bezier(0.0, 0, 0.2, 1)",
  // Objects leave the screen at full velocity. They do not decelerate when off-screen.
  easeIn: "cubic-bezier(0.4, 0, 1, 1)",
  // The sharp curve is used by objects that may return to the screen at any time.
  sharp: "cubic-bezier(0.4, 0, 0.6, 1)"
}, gi = {
  shortest: 150,
  shorter: 200,
  short: 250,
  // most basic recommended timing
  standard: 300,
  // this is to be used in complex animations
  complex: 375,
  // recommended when something is entering screen
  enteringScreen: 225,
  // recommended when something is leaving screen
  leavingScreen: 195
};
function Xs(t) {
  return "".concat(Math.round(t), "ms");
}
const Pp = {
  easing: Ys,
  duration: gi,
  create: function() {
    var e = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : ["all"], a = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {}, r = a.duration, n = r === void 0 ? gi.standard : r, o = a.easing, i = o === void 0 ? Ys.easeInOut : o, s = a.delay, l = s === void 0 ? 0 : s;
    return Ae(a, ["duration", "easing", "delay"]), (Array.isArray(e) ? e : [e]).map(function(u) {
      return "".concat(u, " ").concat(typeof n == "string" ? n : Xs(n), " ").concat(i, " ").concat(typeof l == "string" ? l : Xs(l));
    }).join(",");
  },
  getAutoHeightDuration: function(e) {
    if (!e)
      return 0;
    var a = e / 36;
    return Math.round((4 + 15 * Math.pow(a, 0.25) + a / 5) * 10);
  }
};
var Bc = {
  mobileStepper: 1e3,
  speedDial: 1050,
  appBar: 1100,
  drawer: 1200,
  modal: 1300,
  snackbar: 1400,
  tooltip: 1500
};
function jp() {
  for (var t = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : {}, e = t.breakpoints, a = e === void 0 ? {} : e, r = t.mixins, n = r === void 0 ? {} : r, o = t.palette, i = o === void 0 ? {} : o, s = t.spacing, l = t.typography, u = l === void 0 ? {} : l, c = Ae(t, ["breakpoints", "mixins", "palette", "spacing", "typography"]), d = Yd(i), p = qd(a), h = _p(s), g = wr({
    breakpoints: p,
    direction: "ltr",
    mixins: Gd(p, h, n),
    overrides: {},
    // Inject custom styles
    palette: d,
    props: {},
    // Provide default props
    shadows: tf,
    typography: Jd(d, u),
    spacing: h,
    shape: rf,
    transitions: Pp,
    zIndex: Bc
  }, c), m = arguments.length, y = new Array(m > 1 ? m - 1 : 0), w = 1; w < m; w++)
    y[w - 1] = arguments[w];
  return g = y.reduce(function(k, j) {
    return wr(k, j);
  }, g), g;
}
var Ip = typeof Symbol == "function" && Symbol.for;
const Ap = Ip ? Symbol.for("mui.nested") : "__THEME_NESTED__";
var Mp = ["checked", "disabled", "error", "focused", "focusVisible", "required", "expanded", "selected"];
function Np() {
  var t = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : {}, e = t.disableGlobal, a = e === void 0 ? !1 : e, r = t.productionPrefix, n = r === void 0 ? "jss" : r, o = t.seed, i = o === void 0 ? "" : o, s = i === "" ? "" : "".concat(i, "-"), l = 0, u = function() {
    return l += 1, l;
  };
  return function(c, d) {
    var p = d.options.name;
    if (p && p.indexOf("Mui") === 0 && !d.options.link && !a) {
      if (Mp.indexOf(c.key) !== -1)
        return "Mui-".concat(c.key);
      var h = "".concat(s).concat(p, "-").concat(c.key);
      return !d.options.theme[Ap] || i !== "" ? h : "".concat(h, "-").concat(u());
    }
    return "".concat(s).concat(n).concat(u());
  };
}
function Fc(t) {
  var e = t.theme, a = t.name, r = t.props;
  if (!e || !e.props || !e.props[a])
    return r;
  var n = e.props[a], o;
  for (o in n)
    r[o] === void 0 && (r[o] = n[o]);
  return r;
}
var Js = typeof Symbol == "function" && typeof Symbol.iterator == "symbol" ? function(t) {
  return typeof t;
} : function(t) {
  return t && typeof Symbol == "function" && t.constructor === Symbol && t !== Symbol.prototype ? "symbol" : typeof t;
}, ra = (typeof window > "u" ? "undefined" : Js(window)) === "object" && (typeof document > "u" ? "undefined" : Js(document)) === "object" && document.nodeType === 9;
function Lp(t, e) {
  for (var a = 0; a < e.length; a++) {
    var r = e[a];
    r.enumerable = r.enumerable || !1, r.configurable = !0, "value" in r && (r.writable = !0), Object.defineProperty(t, Oc(r.key), r);
  }
}
function Gi(t, e, a) {
  return e && Lp(t.prototype, e), Object.defineProperty(t, "prototype", {
    writable: !1
  }), t;
}
function vi(t, e) {
  return vi = Object.setPrototypeOf ? Object.setPrototypeOf.bind() : function(a, r) {
    return a.__proto__ = r, a;
  }, vi(t, e);
}
function Ja(t, e) {
  t.prototype = Object.create(e.prototype), t.prototype.constructor = t, vi(t, e);
}
function bi(t) {
  if (t === void 0) throw new ReferenceError("this hasn't been initialised - super() hasn't been called");
  return t;
}
var Dp = {}.constructor;
function yi(t) {
  if (t == null || typeof t != "object") return t;
  if (Array.isArray(t)) return t.map(yi);
  if (t.constructor !== Dp) return t;
  var e = {};
  for (var a in t)
    e[a] = yi(t[a]);
  return e;
}
function Yi(t, e, a) {
  t === void 0 && (t = "unnamed");
  var r = a.jss, n = yi(e), o = r.plugins.onCreateRule(t, n, a);
  return o || (t[0], null);
}
var Zs = function(e, a) {
  for (var r = "", n = 0; n < e.length && e[n] !== "!important"; n++)
    r && (r += a), r += e[n];
  return r;
}, zr = function(e) {
  if (!Array.isArray(e)) return e;
  var a = "";
  if (Array.isArray(e[0]))
    for (var r = 0; r < e.length && e[r] !== "!important"; r++)
      a && (a += ", "), a += Zs(e[r], " ");
  else a = Zs(e, ", ");
  return e[e.length - 1] === "!important" && (a += " !important"), a;
};
function kn(t) {
  return t && t.format === !1 ? {
    linebreak: "",
    space: ""
  } : {
    linebreak: `
`,
    space: " "
  };
}
function jn(t, e) {
  for (var a = "", r = 0; r < e; r++)
    a += "  ";
  return a + t;
}
function qn(t, e, a) {
  a === void 0 && (a = {});
  var r = "";
  if (!e) return r;
  var n = a, o = n.indent, i = o === void 0 ? 0 : o, s = e.fallbacks;
  a.format === !1 && (i = -1 / 0);
  var l = kn(a), u = l.linebreak, c = l.space;
  if (t && i++, s)
    if (Array.isArray(s))
      for (var d = 0; d < s.length; d++) {
        var p = s[d];
        for (var h in p) {
          var g = p[h];
          g != null && (r && (r += u), r += jn(h + ":" + c + zr(g) + ";", i));
        }
      }
    else
      for (var m in s) {
        var y = s[m];
        y != null && (r && (r += u), r += jn(m + ":" + c + zr(y) + ";", i));
      }
  for (var w in e) {
    var k = e[w];
    k != null && w !== "fallbacks" && (r && (r += u), r += jn(w + ":" + c + zr(k) + ";", i));
  }
  return !r && !a.allowEmpty || !t ? r : (i--, r && (r = "" + u + r + u), jn("" + t + c + "{" + r, i) + jn("}", i));
}
var Bp = /([[\].#*$><+~=|^:(),"'`\s])/g, Qs = typeof CSS < "u" && CSS.escape, Xi = (function(t) {
  return Qs ? Qs(t) : t.replace(Bp, "\\$1");
}), zc = /* @__PURE__ */ (function() {
  function t(a, r, n) {
    this.type = "style", this.isProcessed = !1;
    var o = n.sheet, i = n.Renderer;
    this.key = a, this.options = n, this.style = r, o ? this.renderer = o.renderer : i && (this.renderer = new i());
  }
  var e = t.prototype;
  return e.prop = function(r, n, o) {
    if (n === void 0) return this.style[r];
    var i = o ? o.force : !1;
    if (!i && this.style[r] === n) return this;
    var s = n;
    (!o || o.process !== !1) && (s = this.options.jss.plugins.onChangeValue(n, r, this));
    var l = s == null || s === !1, u = r in this.style;
    if (l && !u && !i) return this;
    var c = l && u;
    if (c ? delete this.style[r] : this.style[r] = s, this.renderable && this.renderer)
      return c ? this.renderer.removeProperty(this.renderable, r) : this.renderer.setProperty(this.renderable, r, s), this;
    var d = this.options.sheet;
    return d && d.attached, this;
  }, t;
})(), xi = /* @__PURE__ */ (function(t) {
  Ja(e, t);
  function e(r, n, o) {
    var i;
    i = t.call(this, r, n, o) || this;
    var s = o.selector, l = o.scoped, u = o.sheet, c = o.generateId;
    return s ? i.selectorText = s : l !== !1 && (i.id = c(bi(bi(i)), u), i.selectorText = "." + Xi(i.id)), i;
  }
  var a = e.prototype;
  return a.applyTo = function(n) {
    var o = this.renderer;
    if (o) {
      var i = this.toJSON();
      for (var s in i)
        o.setProperty(n, s, i[s]);
    }
    return this;
  }, a.toJSON = function() {
    var n = {};
    for (var o in this.style) {
      var i = this.style[o];
      typeof i != "object" ? n[o] = i : Array.isArray(i) && (n[o] = zr(i));
    }
    return n;
  }, a.toString = function(n) {
    var o = this.options.sheet, i = o ? o.options.link : !1, s = i ? Y({}, n, {
      allowEmpty: !0
    }) : n;
    return qn(this.selectorText, this.style, s);
  }, Gi(e, [{
    key: "selector",
    set: function(n) {
      if (n !== this.selectorText) {
        this.selectorText = n;
        var o = this.renderer, i = this.renderable;
        if (!(!i || !o)) {
          var s = o.setSelector(i, n);
          s || o.replaceRule(i, this);
        }
      }
    },
    get: function() {
      return this.selectorText;
    }
  }]), e;
})(zc), Fp = {
  onCreateRule: function(e, a, r) {
    return e[0] === "@" || r.parent && r.parent.type === "keyframes" ? null : new xi(e, a, r);
  }
}, Do = {
  indent: 1,
  children: !0
}, zp = /@([\w-]+)/, Wp = /* @__PURE__ */ (function() {
  function t(a, r, n) {
    this.type = "conditional", this.isProcessed = !1, this.key = a;
    var o = a.match(zp);
    this.at = o ? o[1] : "unknown", this.query = n.name || "@" + this.at, this.options = n, this.rules = new Za(Y({}, n, {
      parent: this
    }));
    for (var i in r)
      this.rules.add(i, r[i]);
    this.rules.process();
  }
  var e = t.prototype;
  return e.getRule = function(r) {
    return this.rules.get(r);
  }, e.indexOf = function(r) {
    return this.rules.indexOf(r);
  }, e.addRule = function(r, n, o) {
    var i = this.rules.add(r, n, o);
    return i ? (this.options.jss.plugins.onProcessRule(i), i) : null;
  }, e.replaceRule = function(r, n, o) {
    var i = this.rules.replace(r, n, o);
    return i && this.options.jss.plugins.onProcessRule(i), i;
  }, e.toString = function(r) {
    r === void 0 && (r = Do);
    var n = kn(r), o = n.linebreak;
    if (r.indent == null && (r.indent = Do.indent), r.children == null && (r.children = Do.children), r.children === !1)
      return this.query + " {}";
    var i = this.rules.toString(r);
    return i ? this.query + " {" + o + i + o + "}" : "";
  }, t;
})(), Vp = /@container|@media|@supports\s+/, Hp = {
  onCreateRule: function(e, a, r) {
    return Vp.test(e) ? new Wp(e, a, r) : null;
  }
}, Bo = {
  indent: 1,
  children: !0
}, Kp = /@keyframes\s+([\w-]+)/, Ci = /* @__PURE__ */ (function() {
  function t(a, r, n) {
    this.type = "keyframes", this.at = "@keyframes", this.isProcessed = !1;
    var o = a.match(Kp);
    o && o[1] ? this.name = o[1] : this.name = "noname", this.key = this.type + "-" + this.name, this.options = n;
    var i = n.scoped, s = n.sheet, l = n.generateId;
    this.id = i === !1 ? this.name : Xi(l(this, s)), this.rules = new Za(Y({}, n, {
      parent: this
    }));
    for (var u in r)
      this.rules.add(u, r[u], Y({}, n, {
        parent: this
      }));
    this.rules.process();
  }
  var e = t.prototype;
  return e.toString = function(r) {
    r === void 0 && (r = Bo);
    var n = kn(r), o = n.linebreak;
    if (r.indent == null && (r.indent = Bo.indent), r.children == null && (r.children = Bo.children), r.children === !1)
      return this.at + " " + this.id + " {}";
    var i = this.rules.toString(r);
    return i && (i = "" + o + i + o), this.at + " " + this.id + " {" + i + "}";
  }, t;
})(), Up = /@keyframes\s+/, qp = /\$([\w-]+)/g, Si = function(e, a) {
  return typeof e == "string" ? e.replace(qp, function(r, n) {
    return n in a ? a[n] : r;
  }) : e;
}, el = function(e, a, r) {
  var n = e[a], o = Si(n, r);
  o !== n && (e[a] = o);
}, Gp = {
  onCreateRule: function(e, a, r) {
    return typeof e == "string" && Up.test(e) ? new Ci(e, a, r) : null;
  },
  // Animation name ref replacer.
  onProcessStyle: function(e, a, r) {
    return a.type !== "style" || !r || ("animation-name" in e && el(e, "animation-name", r.keyframes), "animation" in e && el(e, "animation", r.keyframes)), e;
  },
  onChangeValue: function(e, a, r) {
    var n = r.options.sheet;
    if (!n)
      return e;
    switch (a) {
      case "animation":
        return Si(e, n.keyframes);
      case "animation-name":
        return Si(e, n.keyframes);
      default:
        return e;
    }
  }
}, Yp = /* @__PURE__ */ (function(t) {
  Ja(e, t);
  function e() {
    return t.apply(this, arguments) || this;
  }
  var a = e.prototype;
  return a.toString = function(n) {
    var o = this.options.sheet, i = o ? o.options.link : !1, s = i ? Y({}, n, {
      allowEmpty: !0
    }) : n;
    return qn(this.key, this.style, s);
  }, e;
})(zc), Xp = {
  onCreateRule: function(e, a, r) {
    return r.parent && r.parent.type === "keyframes" ? new Yp(e, a, r) : null;
  }
}, Jp = /* @__PURE__ */ (function() {
  function t(a, r, n) {
    this.type = "font-face", this.at = "@font-face", this.isProcessed = !1, this.key = a, this.style = r, this.options = n;
  }
  var e = t.prototype;
  return e.toString = function(r) {
    var n = kn(r), o = n.linebreak;
    if (Array.isArray(this.style)) {
      for (var i = "", s = 0; s < this.style.length; s++)
        i += qn(this.at, this.style[s]), this.style[s + 1] && (i += o);
      return i;
    }
    return qn(this.at, this.style, r);
  }, t;
})(), Zp = /@font-face/, Qp = {
  onCreateRule: function(e, a, r) {
    return Zp.test(e) ? new Jp(e, a, r) : null;
  }
}, eh = /* @__PURE__ */ (function() {
  function t(a, r, n) {
    this.type = "viewport", this.at = "@viewport", this.isProcessed = !1, this.key = a, this.style = r, this.options = n;
  }
  var e = t.prototype;
  return e.toString = function(r) {
    return qn(this.key, this.style, r);
  }, t;
})(), th = {
  onCreateRule: function(e, a, r) {
    return e === "@viewport" || e === "@-ms-viewport" ? new eh(e, a, r) : null;
  }
}, rh = /* @__PURE__ */ (function() {
  function t(a, r, n) {
    this.type = "simple", this.isProcessed = !1, this.key = a, this.value = r, this.options = n;
  }
  var e = t.prototype;
  return e.toString = function(r) {
    if (Array.isArray(this.value)) {
      for (var n = "", o = 0; o < this.value.length; o++)
        n += this.key + " " + this.value[o] + ";", this.value[o + 1] && (n += `
`);
      return n;
    }
    return this.key + " " + this.value + ";";
  }, t;
})(), nh = {
  "@charset": !0,
  "@import": !0,
  "@namespace": !0
}, ah = {
  onCreateRule: function(e, a, r) {
    return e in nh ? new rh(e, a, r) : null;
  }
}, tl = [Fp, Hp, Gp, Xp, Qp, th, ah], oh = {
  process: !0
}, rl = {
  force: !0,
  process: !0
  /**
   * Contains rules objects and allows adding/removing etc.
   * Is used for e.g. by `StyleSheet` or `ConditionalRule`.
   */
}, Za = /* @__PURE__ */ (function() {
  function t(a) {
    this.map = {}, this.raw = {}, this.index = [], this.counter = 0, this.options = a, this.classes = a.classes, this.keyframes = a.keyframes;
  }
  var e = t.prototype;
  return e.add = function(r, n, o) {
    var i = this.options, s = i.parent, l = i.sheet, u = i.jss, c = i.Renderer, d = i.generateId, p = i.scoped, h = Y({
      classes: this.classes,
      parent: s,
      sheet: l,
      jss: u,
      Renderer: c,
      generateId: d,
      scoped: p,
      name: r,
      keyframes: this.keyframes,
      selector: void 0
    }, o), g = r;
    r in this.raw && (g = r + "-d" + this.counter++), this.raw[g] = n, g in this.classes && (h.selector = "." + Xi(this.classes[g]));
    var m = Yi(g, n, h);
    if (!m) return null;
    this.register(m);
    var y = h.index === void 0 ? this.index.length : h.index;
    return this.index.splice(y, 0, m), m;
  }, e.replace = function(r, n, o) {
    var i = this.get(r), s = this.index.indexOf(i);
    i && this.remove(i);
    var l = o;
    return s !== -1 && (l = Y({}, o, {
      index: s
    })), this.add(r, n, l);
  }, e.get = function(r) {
    return this.map[r];
  }, e.remove = function(r) {
    this.unregister(r), delete this.raw[r.key], this.index.splice(this.index.indexOf(r), 1);
  }, e.indexOf = function(r) {
    return this.index.indexOf(r);
  }, e.process = function() {
    var r = this.options.jss.plugins;
    this.index.slice(0).forEach(r.onProcessRule, r);
  }, e.register = function(r) {
    this.map[r.key] = r, r instanceof xi ? (this.map[r.selector] = r, r.id && (this.classes[r.key] = r.id)) : r instanceof Ci && this.keyframes && (this.keyframes[r.name] = r.id);
  }, e.unregister = function(r) {
    delete this.map[r.key], r instanceof xi ? (delete this.map[r.selector], delete this.classes[r.key]) : r instanceof Ci && delete this.keyframes[r.name];
  }, e.update = function() {
    var r, n, o;
    if (typeof (arguments.length <= 0 ? void 0 : arguments[0]) == "string" ? (r = arguments.length <= 0 ? void 0 : arguments[0], n = arguments.length <= 1 ? void 0 : arguments[1], o = arguments.length <= 2 ? void 0 : arguments[2]) : (n = arguments.length <= 0 ? void 0 : arguments[0], o = arguments.length <= 1 ? void 0 : arguments[1], r = null), r)
      this.updateOne(this.get(r), n, o);
    else
      for (var i = 0; i < this.index.length; i++)
        this.updateOne(this.index[i], n, o);
  }, e.updateOne = function(r, n, o) {
    o === void 0 && (o = oh);
    var i = this.options, s = i.jss.plugins, l = i.sheet;
    if (r.rules instanceof t) {
      r.rules.update(n, o);
      return;
    }
    var u = r.style;
    if (s.onUpdate(n, r, l, o), o.process && u && u !== r.style) {
      s.onProcessStyle(r.style, r, l);
      for (var c in r.style) {
        var d = r.style[c], p = u[c];
        d !== p && r.prop(c, d, rl);
      }
      for (var h in u) {
        var g = r.style[h], m = u[h];
        g == null && g !== m && r.prop(h, null, rl);
      }
    }
  }, e.toString = function(r) {
    for (var n = "", o = this.options.sheet, i = o ? o.options.link : !1, s = kn(r), l = s.linebreak, u = 0; u < this.index.length; u++) {
      var c = this.index[u], d = c.toString(r);
      !d && !i || (n && (n += l), n += d);
    }
    return n;
  }, t;
})(), Wc = /* @__PURE__ */ (function() {
  function t(a, r) {
    this.attached = !1, this.deployed = !1, this.classes = {}, this.keyframes = {}, this.options = Y({}, r, {
      sheet: this,
      parent: this,
      classes: this.classes,
      keyframes: this.keyframes
    }), r.Renderer && (this.renderer = new r.Renderer(this)), this.rules = new Za(this.options);
    for (var n in a)
      this.rules.add(n, a[n]);
    this.rules.process();
  }
  var e = t.prototype;
  return e.attach = function() {
    return this.attached ? this : (this.renderer && this.renderer.attach(), this.attached = !0, this.deployed || this.deploy(), this);
  }, e.detach = function() {
    return this.attached ? (this.renderer && this.renderer.detach(), this.attached = !1, this) : this;
  }, e.addRule = function(r, n, o) {
    var i = this.queue;
    this.attached && !i && (this.queue = []);
    var s = this.rules.add(r, n, o);
    return s ? (this.options.jss.plugins.onProcessRule(s), this.attached ? (this.deployed && (i ? i.push(s) : (this.insertRule(s), this.queue && (this.queue.forEach(this.insertRule, this), this.queue = void 0))), s) : (this.deployed = !1, s)) : null;
  }, e.replaceRule = function(r, n, o) {
    var i = this.rules.get(r);
    if (!i) return this.addRule(r, n, o);
    var s = this.rules.replace(r, n, o);
    return s && this.options.jss.plugins.onProcessRule(s), this.attached ? (this.deployed && this.renderer && (s ? i.renderable && this.renderer.replaceRule(i.renderable, s) : this.renderer.deleteRule(i)), s) : (this.deployed = !1, s);
  }, e.insertRule = function(r) {
    this.renderer && this.renderer.insertRule(r);
  }, e.addRules = function(r, n) {
    var o = [];
    for (var i in r) {
      var s = this.addRule(i, r[i], n);
      s && o.push(s);
    }
    return o;
  }, e.getRule = function(r) {
    return this.rules.get(r);
  }, e.deleteRule = function(r) {
    var n = typeof r == "object" ? r : this.rules.get(r);
    return !n || // Style sheet was created without link: true and attached, in this case we
    // won't be able to remove the CSS rule from the DOM.
    this.attached && !n.renderable ? !1 : (this.rules.remove(n), this.attached && n.renderable && this.renderer ? this.renderer.deleteRule(n.renderable) : !0);
  }, e.indexOf = function(r) {
    return this.rules.indexOf(r);
  }, e.deploy = function() {
    return this.renderer && this.renderer.deploy(), this.deployed = !0, this;
  }, e.update = function() {
    var r;
    return (r = this.rules).update.apply(r, arguments), this;
  }, e.updateOne = function(r, n, o) {
    return this.rules.updateOne(r, n, o), this;
  }, e.toString = function(r) {
    return this.rules.toString(r);
  }, t;
})(), ih = /* @__PURE__ */ (function() {
  function t() {
    this.plugins = {
      internal: [],
      external: []
    }, this.registry = {};
  }
  var e = t.prototype;
  return e.onCreateRule = function(r, n, o) {
    for (var i = 0; i < this.registry.onCreateRule.length; i++) {
      var s = this.registry.onCreateRule[i](r, n, o);
      if (s) return s;
    }
    return null;
  }, e.onProcessRule = function(r) {
    if (!r.isProcessed) {
      for (var n = r.options.sheet, o = 0; o < this.registry.onProcessRule.length; o++)
        this.registry.onProcessRule[o](r, n);
      r.style && this.onProcessStyle(r.style, r, n), r.isProcessed = !0;
    }
  }, e.onProcessStyle = function(r, n, o) {
    for (var i = 0; i < this.registry.onProcessStyle.length; i++)
      n.style = this.registry.onProcessStyle[i](n.style, n, o);
  }, e.onProcessSheet = function(r) {
    for (var n = 0; n < this.registry.onProcessSheet.length; n++)
      this.registry.onProcessSheet[n](r);
  }, e.onUpdate = function(r, n, o, i) {
    for (var s = 0; s < this.registry.onUpdate.length; s++)
      this.registry.onUpdate[s](r, n, o, i);
  }, e.onChangeValue = function(r, n, o) {
    for (var i = r, s = 0; s < this.registry.onChangeValue.length; s++)
      i = this.registry.onChangeValue[s](i, n, o);
    return i;
  }, e.use = function(r, n) {
    n === void 0 && (n = {
      queue: "external"
    });
    var o = this.plugins[n.queue];
    o.indexOf(r) === -1 && (o.push(r), this.registry = [].concat(this.plugins.external, this.plugins.internal).reduce(function(i, s) {
      for (var l in s)
        l in i && i[l].push(s[l]);
      return i;
    }, {
      onCreateRule: [],
      onProcessRule: [],
      onProcessStyle: [],
      onProcessSheet: [],
      onChangeValue: [],
      onUpdate: []
    }));
  }, t;
})(), sh = /* @__PURE__ */ (function() {
  function t() {
    this.registry = [];
  }
  var e = t.prototype;
  return e.add = function(r) {
    var n = this.registry, o = r.options.index;
    if (n.indexOf(r) === -1) {
      if (n.length === 0 || o >= this.index) {
        n.push(r);
        return;
      }
      for (var i = 0; i < n.length; i++)
        if (n[i].options.index > o) {
          n.splice(i, 0, r);
          return;
        }
    }
  }, e.reset = function() {
    this.registry = [];
  }, e.remove = function(r) {
    var n = this.registry.indexOf(r);
    this.registry.splice(n, 1);
  }, e.toString = function(r) {
    for (var n = r === void 0 ? {} : r, o = n.attached, i = Ya(n, ["attached"]), s = kn(i), l = s.linebreak, u = "", c = 0; c < this.registry.length; c++) {
      var d = this.registry[c];
      o != null && d.attached !== o || (u && (u += l), u += d.toString(i));
    }
    return u;
  }, Gi(t, [{
    key: "index",
    /**
     * Current highest index number.
     */
    get: function() {
      return this.registry.length === 0 ? 0 : this.registry[this.registry.length - 1].options.index;
    }
  }]), t;
})(), Vn = new sh(), wi = typeof globalThis < "u" ? globalThis : typeof window < "u" && window.Math === Math ? window : typeof self < "u" && self.Math === Math ? self : Function("return this")(), Ei = "2f1acc6c3a606b082e5eef5e54414ffb";
wi[Ei] == null && (wi[Ei] = 0);
var nl = wi[Ei]++, al = function(e) {
  e === void 0 && (e = {});
  var a = 0, r = function(o, i) {
    a += 1;
    var s = "", l = "";
    return i && (i.options.classNamePrefix && (l = i.options.classNamePrefix), i.options.jss.id != null && (s = String(i.options.jss.id))), e.minify ? "" + (l || "c") + nl + s + a : l + o.key + "-" + nl + (s ? "-" + s : "") + "-" + a;
  };
  return r;
}, Vc = function(e) {
  var a;
  return function() {
    return a || (a = e()), a;
  };
}, lh = function(e, a) {
  try {
    return e.attributeStyleMap ? e.attributeStyleMap.get(a) : e.style.getPropertyValue(a);
  } catch {
    return "";
  }
}, ch = function(e, a, r) {
  try {
    var n = r;
    if (Array.isArray(r) && (n = zr(r)), e.attributeStyleMap)
      e.attributeStyleMap.set(a, n);
    else {
      var o = n ? n.indexOf("!important") : -1, i = o > -1 ? n.substr(0, o - 1) : n;
      e.style.setProperty(a, i, o > -1 ? "important" : "");
    }
  } catch {
    return !1;
  }
  return !0;
}, uh = function(e, a) {
  try {
    e.attributeStyleMap ? e.attributeStyleMap.delete(a) : e.style.removeProperty(a);
  } catch {
  }
}, dh = function(e, a) {
  return e.selectorText = a, e.selectorText === a;
}, Hc = Vc(function() {
  return document.querySelector("head");
});
function fh(t, e) {
  for (var a = 0; a < t.length; a++) {
    var r = t[a];
    if (r.attached && r.options.index > e.index && r.options.insertionPoint === e.insertionPoint)
      return r;
  }
  return null;
}
function ph(t, e) {
  for (var a = t.length - 1; a >= 0; a--) {
    var r = t[a];
    if (r.attached && r.options.insertionPoint === e.insertionPoint)
      return r;
  }
  return null;
}
function hh(t) {
  for (var e = Hc(), a = 0; a < e.childNodes.length; a++) {
    var r = e.childNodes[a];
    if (r.nodeType === 8 && r.nodeValue.trim() === t)
      return r;
  }
  return null;
}
function mh(t) {
  var e = Vn.registry;
  if (e.length > 0) {
    var a = fh(e, t);
    if (a && a.renderer)
      return {
        parent: a.renderer.element.parentNode,
        node: a.renderer.element
      };
    if (a = ph(e, t), a && a.renderer)
      return {
        parent: a.renderer.element.parentNode,
        node: a.renderer.element.nextSibling
      };
  }
  var r = t.insertionPoint;
  if (r && typeof r == "string") {
    var n = hh(r);
    if (n)
      return {
        parent: n.parentNode,
        node: n.nextSibling
      };
  }
  return !1;
}
function gh(t, e) {
  var a = e.insertionPoint, r = mh(e);
  if (r !== !1 && r.parent) {
    r.parent.insertBefore(t, r.node);
    return;
  }
  if (a && typeof a.nodeType == "number") {
    var n = a, o = n.parentNode;
    o && o.insertBefore(t, n.nextSibling);
    return;
  }
  Hc().appendChild(t);
}
var vh = Vc(function() {
  var t = document.querySelector('meta[property="csp-nonce"]');
  return t ? t.getAttribute("content") : null;
}), ol = function(e, a, r) {
  try {
    "insertRule" in e ? e.insertRule(a, r) : "appendRule" in e && e.appendRule(a);
  } catch {
    return !1;
  }
  return e.cssRules[r];
}, il = function(e, a) {
  var r = e.cssRules.length;
  return a === void 0 || a > r ? r : a;
}, bh = function() {
  var e = document.createElement("style");
  return e.textContent = `
`, e;
}, yh = /* @__PURE__ */ (function() {
  function t(a) {
    this.getPropertyValue = lh, this.setProperty = ch, this.removeProperty = uh, this.setSelector = dh, this.hasInsertedRules = !1, this.cssRules = [], a && Vn.add(a), this.sheet = a;
    var r = this.sheet ? this.sheet.options : {}, n = r.media, o = r.meta, i = r.element;
    this.element = i || bh(), this.element.setAttribute("data-jss", ""), n && this.element.setAttribute("media", n), o && this.element.setAttribute("data-meta", o);
    var s = vh();
    s && this.element.setAttribute("nonce", s);
  }
  var e = t.prototype;
  return e.attach = function() {
    if (!(this.element.parentNode || !this.sheet)) {
      gh(this.element, this.sheet.options);
      var r = !!(this.sheet && this.sheet.deployed);
      this.hasInsertedRules && r && (this.hasInsertedRules = !1, this.deploy());
    }
  }, e.detach = function() {
    if (this.sheet) {
      var r = this.element.parentNode;
      r && r.removeChild(this.element), this.sheet.options.link && (this.cssRules = [], this.element.textContent = `
`);
    }
  }, e.deploy = function() {
    var r = this.sheet;
    if (r) {
      if (r.options.link) {
        this.insertRules(r.rules);
        return;
      }
      this.element.textContent = `
` + r.toString() + `
`;
    }
  }, e.insertRules = function(r, n) {
    for (var o = 0; o < r.index.length; o++)
      this.insertRule(r.index[o], o, n);
  }, e.insertRule = function(r, n, o) {
    if (o === void 0 && (o = this.element.sheet), r.rules) {
      var i = r, s = o;
      if (r.type === "conditional" || r.type === "keyframes") {
        var l = il(o, n);
        if (s = ol(o, i.toString({
          children: !1
        }), l), s === !1)
          return !1;
        this.refCssRule(r, l, s);
      }
      return this.insertRules(i.rules, s), s;
    }
    var u = r.toString();
    if (!u) return !1;
    var c = il(o, n), d = ol(o, u, c);
    return d === !1 ? !1 : (this.hasInsertedRules = !0, this.refCssRule(r, c, d), d);
  }, e.refCssRule = function(r, n, o) {
    r.renderable = o, r.options.parent instanceof Wc && this.cssRules.splice(n, 0, o);
  }, e.deleteRule = function(r) {
    var n = this.element.sheet, o = this.indexOf(r);
    return o === -1 ? !1 : (n.deleteRule(o), this.cssRules.splice(o, 1), !0);
  }, e.indexOf = function(r) {
    return this.cssRules.indexOf(r);
  }, e.replaceRule = function(r, n) {
    var o = this.indexOf(r);
    return o === -1 ? !1 : (this.element.sheet.deleteRule(o), this.cssRules.splice(o, 1), this.insertRule(n, o));
  }, e.getRules = function() {
    return this.element.sheet.cssRules;
  }, t;
})(), xh = 0, Ch = /* @__PURE__ */ (function() {
  function t(a) {
    this.id = xh++, this.version = "10.10.0", this.plugins = new ih(), this.options = {
      id: {
        minify: !1
      },
      createGenerateId: al,
      Renderer: ra ? yh : null,
      plugins: []
    }, this.generateId = al({
      minify: !1
    });
    for (var r = 0; r < tl.length; r++)
      this.plugins.use(tl[r], {
        queue: "internal"
      });
    this.setup(a);
  }
  var e = t.prototype;
  return e.setup = function(r) {
    return r === void 0 && (r = {}), r.createGenerateId && (this.options.createGenerateId = r.createGenerateId), r.id && (this.options.id = Y({}, this.options.id, r.id)), (r.createGenerateId || r.id) && (this.generateId = this.options.createGenerateId(this.options.id)), r.insertionPoint != null && (this.options.insertionPoint = r.insertionPoint), "Renderer" in r && (this.options.Renderer = r.Renderer), r.plugins && this.use.apply(this, r.plugins), this;
  }, e.createStyleSheet = function(r, n) {
    n === void 0 && (n = {});
    var o = n, i = o.index;
    typeof i != "number" && (i = Vn.index === 0 ? 0 : Vn.index + 1);
    var s = new Wc(r, Y({}, n, {
      jss: this,
      generateId: n.generateId || this.generateId,
      insertionPoint: this.options.insertionPoint,
      Renderer: this.options.Renderer,
      index: i
    }));
    return this.plugins.onProcessSheet(s), s;
  }, e.removeStyleSheet = function(r) {
    return r.detach(), Vn.remove(r), this;
  }, e.createRule = function(r, n, o) {
    if (n === void 0 && (n = {}), o === void 0 && (o = {}), typeof r == "object")
      return this.createRule(void 0, r, n);
    var i = Y({}, o, {
      name: r,
      jss: this,
      Renderer: this.options.Renderer
    });
    i.generateId || (i.generateId = this.generateId), i.classes || (i.classes = {}), i.keyframes || (i.keyframes = {});
    var s = Yi(r, n, i);
    return s && this.plugins.onProcessRule(s), s;
  }, e.use = function() {
    for (var r = this, n = arguments.length, o = new Array(n), i = 0; i < n; i++)
      o[i] = arguments[i];
    return o.forEach(function(s) {
      r.plugins.use(s);
    }), this;
  }, t;
})(), Kc = function(e) {
  return new Ch(e);
}, Ji = typeof CSS == "object" && CSS != null && "number" in CSS;
function Uc(t) {
  var e = null;
  for (var a in t) {
    var r = t[a], n = typeof r;
    if (n === "function")
      e || (e = {}), e[a] = r;
    else if (n === "object" && r !== null && !Array.isArray(r)) {
      var o = Uc(r);
      o && (e || (e = {}), e[a] = o);
    }
  }
  return e;
}
/**
 * A better abstraction over CSS.
 *
 * @copyright Oleg Isonen (Slobodskoi) / Isonen 2014-present
 * @website https://github.com/cssinjs/jss
 * @license MIT
 */
Kc();
var qc = Date.now(), Fo = "fnValues" + qc, zo = "fnStyle" + ++qc, Sh = function() {
  return {
    onCreateRule: function(a, r, n) {
      if (typeof r != "function") return null;
      var o = Yi(a, {}, n);
      return o[zo] = r, o;
    },
    onProcessStyle: function(a, r) {
      if (Fo in r || zo in r) return a;
      var n = {};
      for (var o in a) {
        var i = a[o];
        typeof i == "function" && (delete a[o], n[o] = i);
      }
      return r[Fo] = n, a;
    },
    onUpdate: function(a, r, n, o) {
      var i = r, s = i[zo];
      s && (i.style = s(a) || {});
      var l = i[Fo];
      if (l)
        for (var u in l)
          i.prop(u, l[u](a), o);
    }
  };
}, Cr = "@global", ki = "@global ", wh = /* @__PURE__ */ (function() {
  function t(a, r, n) {
    this.type = "global", this.at = Cr, this.isProcessed = !1, this.key = a, this.options = n, this.rules = new Za(Y({}, n, {
      parent: this
    }));
    for (var o in r)
      this.rules.add(o, r[o]);
    this.rules.process();
  }
  var e = t.prototype;
  return e.getRule = function(r) {
    return this.rules.get(r);
  }, e.addRule = function(r, n, o) {
    var i = this.rules.add(r, n, o);
    return i && this.options.jss.plugins.onProcessRule(i), i;
  }, e.replaceRule = function(r, n, o) {
    var i = this.rules.replace(r, n, o);
    return i && this.options.jss.plugins.onProcessRule(i), i;
  }, e.indexOf = function(r) {
    return this.rules.indexOf(r);
  }, e.toString = function(r) {
    return this.rules.toString(r);
  }, t;
})(), Eh = /* @__PURE__ */ (function() {
  function t(a, r, n) {
    this.type = "global", this.at = Cr, this.isProcessed = !1, this.key = a, this.options = n;
    var o = a.substr(ki.length);
    this.rule = n.jss.createRule(o, r, Y({}, n, {
      parent: this
    }));
  }
  var e = t.prototype;
  return e.toString = function(r) {
    return this.rule ? this.rule.toString(r) : "";
  }, t;
})(), kh = /\s*,\s*/g;
function Gc(t, e) {
  for (var a = t.split(kh), r = "", n = 0; n < a.length; n++)
    r += e + " " + a[n].trim(), a[n + 1] && (r += ", ");
  return r;
}
function $h(t, e) {
  var a = t.options, r = t.style, n = r ? r[Cr] : null;
  if (n) {
    for (var o in n)
      e.addRule(o, n[o], Y({}, a, {
        selector: Gc(o, t.selector)
      }));
    delete r[Cr];
  }
}
function Rh(t, e) {
  var a = t.options, r = t.style;
  for (var n in r)
    if (!(n[0] !== "@" || n.substr(0, Cr.length) !== Cr)) {
      var o = Gc(n.substr(Cr.length), t.selector);
      e.addRule(o, r[n], Y({}, a, {
        selector: o
      })), delete r[n];
    }
}
function Th() {
  function t(a, r, n) {
    if (!a) return null;
    if (a === Cr)
      return new wh(a, r, n);
    if (a[0] === "@" && a.substr(0, ki.length) === ki)
      return new Eh(a, r, n);
    var o = n.parent;
    return o && (o.type === "global" || o.options.parent && o.options.parent.type === "global") && (n.scoped = !1), !n.selector && n.scoped === !1 && (n.selector = a), null;
  }
  function e(a, r) {
    a.type !== "style" || !r || ($h(a, r), Rh(a, r));
  }
  return {
    onCreateRule: t,
    onProcessRule: e
  };
}
var sl = /\s*,\s*/g, Oh = /&/g, _h = /\$([\w-]+)/g;
function Ph() {
  function t(n, o) {
    return function(i, s) {
      var l = n.getRule(s) || o && o.getRule(s);
      return l ? l.selector : s;
    };
  }
  function e(n, o) {
    for (var i = o.split(sl), s = n.split(sl), l = "", u = 0; u < i.length; u++)
      for (var c = i[u], d = 0; d < s.length; d++) {
        var p = s[d];
        l && (l += ", "), l += p.indexOf("&") !== -1 ? p.replace(Oh, c) : c + " " + p;
      }
    return l;
  }
  function a(n, o, i) {
    if (i) return Y({}, i, {
      index: i.index + 1
    });
    var s = n.options.nestingLevel;
    s = s === void 0 ? 1 : s + 1;
    var l = Y({}, n.options, {
      nestingLevel: s,
      index: o.indexOf(n) + 1
      // We don't need the parent name to be set options for chlid.
    });
    return delete l.name, l;
  }
  function r(n, o, i) {
    if (o.type !== "style") return n;
    var s = o, l = s.options.parent, u, c;
    for (var d in n) {
      var p = d.indexOf("&") !== -1, h = d[0] === "@";
      if (!(!p && !h)) {
        if (u = a(s, l, u), p) {
          var g = e(d, s.selector);
          c || (c = t(l, i)), g = g.replace(_h, c);
          var m = s.key + "-" + d;
          "replaceRule" in l ? l.replaceRule(m, n[d], Y({}, u, {
            selector: g
          })) : l.addRule(m, n[d], Y({}, u, {
            selector: g
          }));
        } else h && l.addRule(d, {}, u).addRule(s.key, n[d], {
          selector: s.selector
        });
        delete n[d];
      }
    }
    return n;
  }
  return {
    onProcessStyle: r
  };
}
var jh = /[A-Z]/g, Ih = /^ms-/, Wo = {};
function Ah(t) {
  return "-" + t.toLowerCase();
}
function Yc(t) {
  if (Wo.hasOwnProperty(t))
    return Wo[t];
  var e = t.replace(jh, Ah);
  return Wo[t] = Ih.test(e) ? "-" + e : e;
}
function za(t) {
  var e = {};
  for (var a in t) {
    var r = a.indexOf("--") === 0 ? a : Yc(a);
    e[r] = t[a];
  }
  return t.fallbacks && (Array.isArray(t.fallbacks) ? e.fallbacks = t.fallbacks.map(za) : e.fallbacks = za(t.fallbacks)), e;
}
function Mh() {
  function t(a) {
    if (Array.isArray(a)) {
      for (var r = 0; r < a.length; r++)
        a[r] = za(a[r]);
      return a;
    }
    return za(a);
  }
  function e(a, r, n) {
    if (r.indexOf("--") === 0)
      return a;
    var o = Yc(r);
    return r === o ? a : (n.prop(o, a), null);
  }
  return {
    onProcessStyle: t,
    onChangeValue: e
  };
}
var ae = Ji && CSS ? CSS.px : "px", ba = Ji && CSS ? CSS.ms : "ms", nn = Ji && CSS ? CSS.percent : "%", Nh = {
  // Animation properties
  "animation-delay": ba,
  "animation-duration": ba,
  // Background properties
  "background-position": ae,
  "background-position-x": ae,
  "background-position-y": ae,
  "background-size": ae,
  // Border Properties
  border: ae,
  "border-bottom": ae,
  "border-bottom-left-radius": ae,
  "border-bottom-right-radius": ae,
  "border-bottom-width": ae,
  "border-left": ae,
  "border-left-width": ae,
  "border-radius": ae,
  "border-right": ae,
  "border-right-width": ae,
  "border-top": ae,
  "border-top-left-radius": ae,
  "border-top-right-radius": ae,
  "border-top-width": ae,
  "border-width": ae,
  "border-block": ae,
  "border-block-end": ae,
  "border-block-end-width": ae,
  "border-block-start": ae,
  "border-block-start-width": ae,
  "border-block-width": ae,
  "border-inline": ae,
  "border-inline-end": ae,
  "border-inline-end-width": ae,
  "border-inline-start": ae,
  "border-inline-start-width": ae,
  "border-inline-width": ae,
  "border-start-start-radius": ae,
  "border-start-end-radius": ae,
  "border-end-start-radius": ae,
  "border-end-end-radius": ae,
  // Margin properties
  margin: ae,
  "margin-bottom": ae,
  "margin-left": ae,
  "margin-right": ae,
  "margin-top": ae,
  "margin-block": ae,
  "margin-block-end": ae,
  "margin-block-start": ae,
  "margin-inline": ae,
  "margin-inline-end": ae,
  "margin-inline-start": ae,
  // Padding properties
  padding: ae,
  "padding-bottom": ae,
  "padding-left": ae,
  "padding-right": ae,
  "padding-top": ae,
  "padding-block": ae,
  "padding-block-end": ae,
  "padding-block-start": ae,
  "padding-inline": ae,
  "padding-inline-end": ae,
  "padding-inline-start": ae,
  // Mask properties
  "mask-position-x": ae,
  "mask-position-y": ae,
  "mask-size": ae,
  // Width and height properties
  height: ae,
  width: ae,
  "min-height": ae,
  "max-height": ae,
  "min-width": ae,
  "max-width": ae,
  // Position properties
  bottom: ae,
  left: ae,
  top: ae,
  right: ae,
  inset: ae,
  "inset-block": ae,
  "inset-block-end": ae,
  "inset-block-start": ae,
  "inset-inline": ae,
  "inset-inline-end": ae,
  "inset-inline-start": ae,
  // Shadow properties
  "box-shadow": ae,
  "text-shadow": ae,
  // Column properties
  "column-gap": ae,
  "column-rule": ae,
  "column-rule-width": ae,
  "column-width": ae,
  // Font and text properties
  "font-size": ae,
  "font-size-delta": ae,
  "letter-spacing": ae,
  "text-decoration-thickness": ae,
  "text-indent": ae,
  "text-stroke": ae,
  "text-stroke-width": ae,
  "word-spacing": ae,
  // Motion properties
  motion: ae,
  "motion-offset": ae,
  // Outline properties
  outline: ae,
  "outline-offset": ae,
  "outline-width": ae,
  // Perspective properties
  perspective: ae,
  "perspective-origin-x": nn,
  "perspective-origin-y": nn,
  // Transform properties
  "transform-origin": nn,
  "transform-origin-x": nn,
  "transform-origin-y": nn,
  "transform-origin-z": nn,
  // Transition properties
  "transition-delay": ba,
  "transition-duration": ba,
  // Alignment properties
  "vertical-align": ae,
  "flex-basis": ae,
  // Some random properties
  "shape-margin": ae,
  size: ae,
  gap: ae,
  // Grid properties
  grid: ae,
  "grid-gap": ae,
  "row-gap": ae,
  "grid-row-gap": ae,
  "grid-column-gap": ae,
  "grid-template-rows": ae,
  "grid-template-columns": ae,
  "grid-auto-rows": ae,
  "grid-auto-columns": ae,
  // Not existing properties.
  // Used to avoid issues with jss-plugin-expand integration.
  "box-shadow-x": ae,
  "box-shadow-y": ae,
  "box-shadow-blur": ae,
  "box-shadow-spread": ae,
  "font-line-height": ae,
  "text-shadow-x": ae,
  "text-shadow-y": ae,
  "text-shadow-blur": ae
};
function Xc(t) {
  var e = /(-[a-z])/g, a = function(i) {
    return i[1].toUpperCase();
  }, r = {};
  for (var n in t)
    r[n] = t[n], r[n.replace(e, a)] = t[n];
  return r;
}
var Lh = Xc(Nh);
function Hn(t, e, a) {
  if (e == null) return e;
  if (Array.isArray(e))
    for (var r = 0; r < e.length; r++)
      e[r] = Hn(t, e[r], a);
  else if (typeof e == "object")
    if (t === "fallbacks")
      for (var n in e)
        e[n] = Hn(n, e[n], a);
    else
      for (var o in e)
        e[o] = Hn(t + "-" + o, e[o], a);
  else if (typeof e == "number" && isNaN(e) === !1) {
    var i = a[t] || Lh[t];
    return i && !(e === 0 && i === ae) ? typeof i == "function" ? i(e).toString() : "" + e + i : e.toString();
  }
  return e;
}
function Dh(t) {
  t === void 0 && (t = {});
  var e = Xc(t);
  function a(n, o) {
    if (o.type !== "style") return n;
    for (var i in n)
      n[i] = Hn(i, n[i], e);
    return n;
  }
  function r(n, o) {
    return Hn(o, n, e);
  }
  return {
    onProcessStyle: a,
    onChangeValue: r
  };
}
var Ln = "", $i = "", Jc = "", Zc = "", Bh = ra && "ontouchstart" in document.documentElement;
if (ra) {
  var Vo = {
    Moz: "-moz-",
    ms: "-ms-",
    O: "-o-",
    Webkit: "-webkit-"
  }, Fh = document.createElement("p"), Ho = Fh.style, zh = "Transform";
  for (var Ko in Vo)
    if (Ko + zh in Ho) {
      Ln = Ko, $i = Vo[Ko];
      break;
    }
  Ln === "Webkit" && "msHyphens" in Ho && (Ln = "ms", $i = Vo.ms, Zc = "edge"), Ln === "Webkit" && "-apple-trailing-word" in Ho && (Jc = "apple");
}
var Ke = {
  js: Ln,
  css: $i,
  vendor: Jc,
  browser: Zc,
  isTouch: Bh
};
function Wh(t) {
  return t[1] === "-" || Ke.js === "ms" ? t : "@" + Ke.css + "keyframes" + t.substr(10);
}
var Vh = {
  noPrefill: ["appearance"],
  supportedProperty: function(e) {
    return e !== "appearance" ? !1 : Ke.js === "ms" ? "-webkit-" + e : Ke.css + e;
  }
}, Hh = {
  noPrefill: ["color-adjust"],
  supportedProperty: function(e) {
    return e !== "color-adjust" ? !1 : Ke.js === "Webkit" ? Ke.css + "print-" + e : e;
  }
}, Kh = /[-\s]+(.)?/g;
function Uh(t, e) {
  return e ? e.toUpperCase() : "";
}
function Zi(t) {
  return t.replace(Kh, Uh);
}
function Er(t) {
  return Zi("-" + t);
}
var qh = {
  noPrefill: ["mask"],
  supportedProperty: function(e, a) {
    if (!/^mask/.test(e)) return !1;
    if (Ke.js === "Webkit") {
      var r = "mask-image";
      if (Zi(r) in a)
        return e;
      if (Ke.js + Er(r) in a)
        return Ke.css + e;
    }
    return e;
  }
}, Gh = {
  noPrefill: ["text-orientation"],
  supportedProperty: function(e) {
    return e !== "text-orientation" ? !1 : Ke.vendor === "apple" && !Ke.isTouch ? Ke.css + e : e;
  }
}, Yh = {
  noPrefill: ["transform"],
  supportedProperty: function(e, a, r) {
    return e !== "transform" ? !1 : r.transform ? e : Ke.css + e;
  }
}, Xh = {
  noPrefill: ["transition"],
  supportedProperty: function(e, a, r) {
    return e !== "transition" ? !1 : r.transition ? e : Ke.css + e;
  }
}, Jh = {
  noPrefill: ["writing-mode"],
  supportedProperty: function(e) {
    return e !== "writing-mode" ? !1 : Ke.js === "Webkit" || Ke.js === "ms" && Ke.browser !== "edge" ? Ke.css + e : e;
  }
}, Zh = {
  noPrefill: ["user-select"],
  supportedProperty: function(e) {
    return e !== "user-select" ? !1 : Ke.js === "Moz" || Ke.js === "ms" || Ke.vendor === "apple" ? Ke.css + e : e;
  }
}, Qh = {
  supportedProperty: function(e, a) {
    if (!/^break-/.test(e)) return !1;
    if (Ke.js === "Webkit") {
      var r = "WebkitColumn" + Er(e);
      return r in a ? Ke.css + "column-" + e : !1;
    }
    if (Ke.js === "Moz") {
      var n = "page" + Er(e);
      return n in a ? "page-" + e : !1;
    }
    return !1;
  }
}, em = {
  supportedProperty: function(e, a) {
    if (!/^(border|margin|padding)-inline/.test(e)) return !1;
    if (Ke.js === "Moz") return e;
    var r = e.replace("-inline", "");
    return Ke.js + Er(r) in a ? Ke.css + r : !1;
  }
}, tm = {
  supportedProperty: function(e, a) {
    return Zi(e) in a ? e : !1;
  }
}, rm = {
  supportedProperty: function(e, a) {
    var r = Er(e);
    return e[0] === "-" || e[0] === "-" && e[1] === "-" ? e : Ke.js + r in a ? Ke.css + e : Ke.js !== "Webkit" && "Webkit" + r in a ? "-webkit-" + e : !1;
  }
}, nm = {
  supportedProperty: function(e) {
    return e.substring(0, 11) !== "scroll-snap" ? !1 : Ke.js === "ms" ? "" + Ke.css + e : e;
  }
}, am = {
  supportedProperty: function(e) {
    return e !== "overscroll-behavior" ? !1 : Ke.js === "ms" ? Ke.css + "scroll-chaining" : e;
  }
}, om = {
  "flex-grow": "flex-positive",
  "flex-shrink": "flex-negative",
  "flex-basis": "flex-preferred-size",
  "justify-content": "flex-pack",
  order: "flex-order",
  "align-items": "flex-align",
  "align-content": "flex-line-pack"
  // 'align-self' is handled by 'align-self' plugin.
}, im = {
  supportedProperty: function(e, a) {
    var r = om[e];
    return r && Ke.js + Er(r) in a ? Ke.css + r : !1;
  }
}, Qc = {
  flex: "box-flex",
  "flex-grow": "box-flex",
  "flex-direction": ["box-orient", "box-direction"],
  order: "box-ordinal-group",
  "align-items": "box-align",
  "flex-flow": ["box-orient", "box-direction"],
  "justify-content": "box-pack"
}, sm = Object.keys(Qc), lm = function(e) {
  return Ke.css + e;
}, cm = {
  supportedProperty: function(e, a, r) {
    var n = r.multiple;
    if (sm.indexOf(e) > -1) {
      var o = Qc[e];
      if (!Array.isArray(o))
        return Ke.js + Er(o) in a ? Ke.css + o : !1;
      if (!n) return !1;
      for (var i = 0; i < o.length; i++)
        if (!(Ke.js + Er(o[0]) in a))
          return !1;
      return o.map(lm);
    }
    return !1;
  }
}, eu = [Vh, Hh, qh, Gh, Yh, Xh, Jh, Zh, Qh, em, tm, rm, nm, am, im, cm], ll = eu.filter(function(t) {
  return t.supportedProperty;
}).map(function(t) {
  return t.supportedProperty;
}), um = eu.filter(function(t) {
  return t.noPrefill;
}).reduce(function(t, e) {
  return t.push.apply(t, Xa(e.noPrefill)), t;
}, []), Dn, Lr = {};
if (ra) {
  Dn = document.createElement("p");
  var Uo = window.getComputedStyle(document.documentElement, "");
  for (var qo in Uo)
    isNaN(qo) || (Lr[Uo[qo]] = Uo[qo]);
  um.forEach(function(t) {
    return delete Lr[t];
  });
}
function Ri(t, e) {
  if (e === void 0 && (e = {}), !Dn) return t;
  if (Lr[t] != null)
    return Lr[t];
  (t === "transition" || t === "transform") && (e[t] = t in Dn.style);
  for (var a = 0; a < ll.length && (Lr[t] = ll[a](t, Dn.style, e), !Lr[t]); a++)
    ;
  try {
    Dn.style[t] = "";
  } catch {
    return !1;
  }
  return Lr[t];
}
var an = {}, dm = {
  transition: 1,
  "transition-property": 1,
  "-webkit-transition": 1,
  "-webkit-transition-property": 1
}, fm = /(^\s*[\w-]+)|, (\s*[\w-]+)(?![^()]*\))/g, br;
function pm(t, e, a) {
  if (e === "var") return "var";
  if (e === "all") return "all";
  if (a === "all") return ", all";
  var r = e ? Ri(e) : ", " + Ri(a);
  return r || e || a;
}
ra && (br = document.createElement("p"));
function cl(t, e) {
  var a = e;
  if (!br || t === "content") return e;
  if (typeof a != "string" || !isNaN(parseInt(a, 10)))
    return a;
  var r = t + a;
  if (an[r] != null)
    return an[r];
  try {
    br.style[t] = a;
  } catch {
    return an[r] = !1, !1;
  }
  if (dm[t])
    a = a.replace(fm, pm);
  else if (br.style[t] === "" && (a = Ke.css + a, a === "-ms-flex" && (br.style[t] = "-ms-flexbox"), br.style[t] = a, br.style[t] === ""))
    return an[r] = !1, !1;
  return br.style[t] = "", an[r] = a, an[r];
}
function hm() {
  function t(n) {
    if (n.type === "keyframes") {
      var o = n;
      o.at = Wh(o.at);
    }
  }
  function e(n) {
    for (var o in n) {
      var i = n[o];
      if (o === "fallbacks" && Array.isArray(i)) {
        n[o] = i.map(e);
        continue;
      }
      var s = !1, l = Ri(o);
      l && l !== o && (s = !0);
      var u = !1, c = cl(l, zr(i));
      c && c !== i && (u = !0), (s || u) && (s && delete n[o], n[l || o] = c || i);
    }
    return n;
  }
  function a(n, o) {
    return o.type !== "style" ? n : e(n);
  }
  function r(n, o) {
    return cl(o, zr(n)) || n;
  }
  return {
    onProcessRule: t,
    onProcessStyle: a,
    onChangeValue: r
  };
}
function mm() {
  var t = function(a, r) {
    return a.length === r.length ? a > r ? 1 : -1 : a.length - r.length;
  };
  return {
    onProcessStyle: function(a, r) {
      if (r.type !== "style") return a;
      for (var n = {}, o = Object.keys(a).sort(t), i = 0; i < o.length; i++)
        n[o[i]] = a[o[i]];
      return n;
    }
  };
}
function gm() {
  return {
    plugins: [
      Sh(),
      Th(),
      Ph(),
      Mh(),
      Dh(),
      // Disable the vendor prefixer server-side, it does nothing.
      // This way, we can get a performance boost.
      // In the documentation, we are using `autoprefixer` to solve this problem.
      typeof window > "u" ? null : hm(),
      mm()
    ]
  };
}
function Qi() {
  var t = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : {}, e = t.baseClasses, a = t.newClasses;
  if (t.Component, !a)
    return e;
  var r = Y({}, e);
  return Object.keys(a).forEach(function(n) {
    a[n] && (r[n] = "".concat(e[n], " ").concat(a[n]));
  }), r;
}
var hn = {
  set: function(e, a, r, n) {
    var o = e.get(a);
    o || (o = /* @__PURE__ */ new Map(), e.set(a, o)), o.set(r, n);
  },
  get: function(e, a, r) {
    var n = e.get(a);
    return n ? n.get(r) : void 0;
  },
  delete: function(e, a, r) {
    var n = e.get(a);
    n.delete(r);
  }
}, vm = ht.createContext(null);
function na() {
  var t = ht.useContext(vm);
  return t;
}
var bm = Kc(gm()), ym = Np(), xm = /* @__PURE__ */ new Map(), Cm = {
  disableGeneration: !1,
  generateClassName: ym,
  jss: bm,
  sheetsCache: null,
  sheetsManager: xm,
  sheetsRegistry: null
}, Sm = ht.createContext(Cm), ul = -1e9;
function wm() {
  return ul += 1, ul;
}
var Em = {};
function km(t) {
  var e = typeof t == "function";
  return {
    create: function(r, n) {
      var o;
      try {
        o = e ? t(r) : t;
      } catch (l) {
        throw l;
      }
      if (!n || !r.overrides || !r.overrides[n])
        return o;
      var i = r.overrides[n], s = Y({}, o);
      return Object.keys(i).forEach(function(l) {
        s[l] = wr(s[l], i[l]);
      }), s;
    },
    options: {}
  };
}
function $m(t, e, a) {
  var r = t.state, n = t.stylesOptions;
  if (n.disableGeneration)
    return e || {};
  r.cacheClasses || (r.cacheClasses = {
    // Cache for the finalized classes value.
    value: null,
    // Cache for the last used classes prop pointer.
    lastProp: null,
    // Cache for the last used rendered classes pointer.
    lastJSS: {}
  });
  var o = !1;
  return r.classes !== r.cacheClasses.lastJSS && (r.cacheClasses.lastJSS = r.classes, o = !0), e !== r.cacheClasses.lastProp && (r.cacheClasses.lastProp = e, o = !0), o && (r.cacheClasses.value = Qi({
    baseClasses: r.cacheClasses.lastJSS,
    newClasses: e,
    Component: a
  })), r.cacheClasses.value;
}
function Rm(t, e) {
  var a = t.state, r = t.theme, n = t.stylesOptions, o = t.stylesCreator, i = t.name;
  if (!n.disableGeneration) {
    var s = hn.get(n.sheetsManager, o, r);
    s || (s = {
      refs: 0,
      staticSheet: null,
      dynamicStyles: null
    }, hn.set(n.sheetsManager, o, r, s));
    var l = Y({}, o.options, n, {
      theme: r,
      flip: typeof n.flip == "boolean" ? n.flip : r.direction === "rtl"
    });
    l.generateId = l.serverGenerateClassName || l.generateClassName;
    var u = n.sheetsRegistry;
    if (s.refs === 0) {
      var c;
      n.sheetsCache && (c = hn.get(n.sheetsCache, o, r));
      var d = o.create(r, i);
      c || (c = n.jss.createStyleSheet(d, Y({
        link: !1
      }, l)), c.attach(), n.sheetsCache && hn.set(n.sheetsCache, o, r, c)), u && u.add(c), s.staticSheet = c, s.dynamicStyles = Uc(d);
    }
    if (s.dynamicStyles) {
      var p = n.jss.createStyleSheet(s.dynamicStyles, Y({
        link: !0
      }, l));
      p.update(e), p.attach(), a.dynamicSheet = p, a.classes = Qi({
        baseClasses: s.staticSheet.classes,
        newClasses: p.classes
      }), u && u.add(p);
    } else
      a.classes = s.staticSheet.classes;
    s.refs += 1;
  }
}
function Tm(t, e) {
  var a = t.state;
  a.dynamicSheet && a.dynamicSheet.update(e);
}
function Om(t) {
  var e = t.state, a = t.theme, r = t.stylesOptions, n = t.stylesCreator;
  if (!r.disableGeneration) {
    var o = hn.get(r.sheetsManager, n, a);
    o.refs -= 1;
    var i = r.sheetsRegistry;
    o.refs === 0 && (hn.delete(r.sheetsManager, n, a), r.jss.removeStyleSheet(o.staticSheet), i && i.remove(o.staticSheet)), e.dynamicSheet && (r.jss.removeStyleSheet(e.dynamicSheet), i && i.remove(e.dynamicSheet));
  }
}
function _m(t, e) {
  var a = ht.useRef([]), r, n = ht.useMemo(function() {
    return {};
  }, e);
  a.current !== n && (a.current = n, r = t()), ht.useEffect(
    function() {
      return function() {
        r && r();
      };
    },
    [n]
    // eslint-disable-line react-hooks/exhaustive-deps
  );
}
function es(t) {
  var e = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {}, a = e.name, r = e.classNamePrefix, n = e.Component, o = e.defaultTheme, i = o === void 0 ? Em : o, s = Ae(e, ["name", "classNamePrefix", "Component", "defaultTheme"]), l = km(t), u = a || r || "makeStyles";
  l.options = {
    index: wm(),
    name: a,
    meta: u,
    classNamePrefix: u
  };
  var c = function() {
    var p = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : {}, h = na() || i, g = Y({}, ht.useContext(Sm), s), m = ht.useRef(), y = ht.useRef();
    _m(function() {
      var k = {
        name: a,
        state: {},
        stylesCreator: l,
        stylesOptions: g,
        theme: h
      };
      return Rm(k, p), y.current = !1, m.current = k, function() {
        Om(k);
      };
    }, [h, l]), ht.useEffect(function() {
      y.current && Tm(m.current, p), y.current = !0;
    });
    var w = $m(m.current, p.classes, n);
    return w;
  };
  return c;
}
function Pm(t, e) {
  if (!(t instanceof e)) throw new TypeError("Cannot call a class as a function");
}
function tu(t) {
  var e, a, r = "";
  if (typeof t == "string" || typeof t == "number") r += t;
  else if (typeof t == "object") if (Array.isArray(t)) for (e = 0; e < t.length; e++) t[e] && (a = tu(t[e])) && (r && (r += " "), r += a);
  else for (e in t) t[e] && (r && (r += " "), r += e);
  return r;
}
function he() {
  for (var t, e, a = 0, r = ""; a < arguments.length; ) (t = arguments[a++]) && (e = tu(t)) && (r && (r += " "), r += e);
  return r;
}
var Go = { exports: {} }, ut = {};
/** @license React v16.13.1
 * react-is.production.min.js
 *
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var dl;
function jm() {
  if (dl) return ut;
  dl = 1;
  var t = typeof Symbol == "function" && Symbol.for, e = t ? Symbol.for("react.element") : 60103, a = t ? Symbol.for("react.portal") : 60106, r = t ? Symbol.for("react.fragment") : 60107, n = t ? Symbol.for("react.strict_mode") : 60108, o = t ? Symbol.for("react.profiler") : 60114, i = t ? Symbol.for("react.provider") : 60109, s = t ? Symbol.for("react.context") : 60110, l = t ? Symbol.for("react.async_mode") : 60111, u = t ? Symbol.for("react.concurrent_mode") : 60111, c = t ? Symbol.for("react.forward_ref") : 60112, d = t ? Symbol.for("react.suspense") : 60113, p = t ? Symbol.for("react.suspense_list") : 60120, h = t ? Symbol.for("react.memo") : 60115, g = t ? Symbol.for("react.lazy") : 60116, m = t ? Symbol.for("react.block") : 60121, y = t ? Symbol.for("react.fundamental") : 60117, w = t ? Symbol.for("react.responder") : 60118, k = t ? Symbol.for("react.scope") : 60119;
  function j(x) {
    if (typeof x == "object" && x !== null) {
      var E = x.$$typeof;
      switch (E) {
        case e:
          switch (x = x.type, x) {
            case l:
            case u:
            case r:
            case o:
            case n:
            case d:
              return x;
            default:
              switch (x = x && x.$$typeof, x) {
                case s:
                case c:
                case g:
                case h:
                case i:
                  return x;
                default:
                  return E;
              }
          }
        case a:
          return E;
      }
    }
  }
  function f(x) {
    return j(x) === u;
  }
  return ut.AsyncMode = l, ut.ConcurrentMode = u, ut.ContextConsumer = s, ut.ContextProvider = i, ut.Element = e, ut.ForwardRef = c, ut.Fragment = r, ut.Lazy = g, ut.Memo = h, ut.Portal = a, ut.Profiler = o, ut.StrictMode = n, ut.Suspense = d, ut.isAsyncMode = function(x) {
    return f(x) || j(x) === l;
  }, ut.isConcurrentMode = f, ut.isContextConsumer = function(x) {
    return j(x) === s;
  }, ut.isContextProvider = function(x) {
    return j(x) === i;
  }, ut.isElement = function(x) {
    return typeof x == "object" && x !== null && x.$$typeof === e;
  }, ut.isForwardRef = function(x) {
    return j(x) === c;
  }, ut.isFragment = function(x) {
    return j(x) === r;
  }, ut.isLazy = function(x) {
    return j(x) === g;
  }, ut.isMemo = function(x) {
    return j(x) === h;
  }, ut.isPortal = function(x) {
    return j(x) === a;
  }, ut.isProfiler = function(x) {
    return j(x) === o;
  }, ut.isStrictMode = function(x) {
    return j(x) === n;
  }, ut.isSuspense = function(x) {
    return j(x) === d;
  }, ut.isValidElementType = function(x) {
    return typeof x == "string" || typeof x == "function" || x === r || x === u || x === o || x === n || x === d || x === p || typeof x == "object" && x !== null && (x.$$typeof === g || x.$$typeof === h || x.$$typeof === i || x.$$typeof === s || x.$$typeof === c || x.$$typeof === y || x.$$typeof === w || x.$$typeof === k || x.$$typeof === m);
  }, ut.typeOf = j, ut;
}
var fl;
function Im() {
  return fl || (fl = 1, Go.exports = jm()), Go.exports;
}
var Yo, pl;
function Am() {
  if (pl) return Yo;
  pl = 1;
  var t = Im(), e = {
    childContextTypes: !0,
    contextType: !0,
    contextTypes: !0,
    defaultProps: !0,
    displayName: !0,
    getDefaultProps: !0,
    getDerivedStateFromError: !0,
    getDerivedStateFromProps: !0,
    mixins: !0,
    propTypes: !0,
    type: !0
  }, a = {
    name: !0,
    length: !0,
    prototype: !0,
    caller: !0,
    callee: !0,
    arguments: !0,
    arity: !0
  }, r = {
    $$typeof: !0,
    render: !0,
    defaultProps: !0,
    displayName: !0,
    propTypes: !0
  }, n = {
    $$typeof: !0,
    compare: !0,
    defaultProps: !0,
    displayName: !0,
    propTypes: !0,
    type: !0
  }, o = {};
  o[t.ForwardRef] = r, o[t.Memo] = n;
  function i(g) {
    return t.isMemo(g) ? n : o[g.$$typeof] || e;
  }
  var s = Object.defineProperty, l = Object.getOwnPropertyNames, u = Object.getOwnPropertySymbols, c = Object.getOwnPropertyDescriptor, d = Object.getPrototypeOf, p = Object.prototype;
  function h(g, m, y) {
    if (typeof m != "string") {
      if (p) {
        var w = d(m);
        w && w !== p && h(g, w, y);
      }
      var k = l(m);
      u && (k = k.concat(u(m)));
      for (var j = i(g), f = i(m), x = 0; x < k.length; ++x) {
        var E = k[x];
        if (!a[E] && !(y && y[E]) && !(f && f[E]) && !(j && j[E])) {
          var L = c(m, E);
          try {
            s(g, E, L);
          } catch {
          }
        }
      }
    }
    return g;
  }
  return Yo = h, Yo;
}
var Mm = Am();
const ru = /* @__PURE__ */ Tr(Mm);
function Nm(t, e) {
  var a = {};
  return Object.keys(t).forEach(function(r) {
    e.indexOf(r) === -1 && (a[r] = t[r]);
  }), a;
}
function Lm(t) {
  var e = function(r) {
    var n = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {}, o = n.name, i = Ae(n, ["name"]), s = o, l = typeof r == "function" ? function(p) {
      return {
        root: function(g) {
          return r(Y({
            theme: p
          }, g));
        }
      };
    } : {
      root: r
    }, u = es(l, Y({
      Component: t,
      name: o || t.displayName,
      classNamePrefix: s
    }, i)), c;
    r.filterProps && (c = r.filterProps, delete r.filterProps), r.propTypes && (r.propTypes, delete r.propTypes);
    var d = /* @__PURE__ */ ht.forwardRef(function(h, g) {
      var m = h.children, y = h.className, w = h.clone, k = h.component, j = Ae(h, ["children", "className", "clone", "component"]), f = u(h), x = he(f.root, y), E = j;
      if (c && (E = Nm(E, c)), w)
        return /* @__PURE__ */ ht.cloneElement(m, Y({
          className: he(m.props.className, x)
        }, E));
      if (typeof m == "function")
        return m(Y({
          className: x
        }, E));
      var L = k || t;
      return /* @__PURE__ */ ht.createElement(L, Y({
        ref: g,
        className: x
      }, E), m);
    });
    return ru(d, t), d;
  };
  return e;
}
var Dm = function(e) {
  var a = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {};
  return function(r) {
    var n = a.defaultTheme, o = a.withTheme, i = o === void 0 ? !1 : o, s = a.name, l = Ae(a, ["defaultTheme", "withTheme", "name"]), u = s, c = es(e, Y({
      defaultTheme: n,
      Component: r,
      name: s || r.displayName,
      classNamePrefix: u
    }, l)), d = /* @__PURE__ */ ht.forwardRef(function(h, g) {
      h.classes;
      var m = h.innerRef, y = Ae(h, ["classes", "innerRef"]), w = c(Y({}, r.defaultProps, h)), k, j = y;
      return (typeof s == "string" || i) && (k = na() || n, s && (j = Fc({
        theme: k,
        name: s,
        props: y
      })), i && !j.theme && (j.theme = k)), /* @__PURE__ */ ht.createElement(r, Y({
        ref: m || g,
        classes: w
      }, j));
    });
    return ru(d, r), d;
  };
};
function fr(t) {
  return t;
}
var Qa = jp();
function ar(t) {
  var e = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {};
  return es(t, Y({
    defaultTheme: Qa
  }, e));
}
var ts = function(e) {
  var a = Lm(e);
  return function(r, n) {
    return a(r, Y({
      defaultTheme: Qa
    }, n));
  };
};
function $n() {
  var t = na() || Qa;
  return t;
}
function et(t, e) {
  return Dm(t, Y({
    defaultTheme: Qa
  }, e));
}
function pt(t) {
  if (typeof t != "string")
    throw new Error(vn(7));
  return t.charAt(0).toUpperCase() + t.slice(1);
}
function Gn() {
  for (var t = arguments.length, e = new Array(t), a = 0; a < t; a++)
    e[a] = arguments[a];
  return e.reduce(function(r, n) {
    return n == null ? r : function() {
      for (var i = arguments.length, s = new Array(i), l = 0; l < i; l++)
        s[l] = arguments[l];
      r.apply(this, s), n.apply(this, s);
    };
  }, function() {
  });
}
var Bm = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      userSelect: "none",
      width: "1em",
      height: "1em",
      display: "inline-block",
      fill: "currentColor",
      flexShrink: 0,
      fontSize: e.typography.pxToRem(24),
      transition: e.transitions.create("fill", {
        duration: e.transitions.duration.shorter
      })
    },
    /* Styles applied to the root element if `color="primary"`. */
    colorPrimary: {
      color: e.palette.primary.main
    },
    /* Styles applied to the root element if `color="secondary"`. */
    colorSecondary: {
      color: e.palette.secondary.main
    },
    /* Styles applied to the root element if `color="action"`. */
    colorAction: {
      color: e.palette.action.active
    },
    /* Styles applied to the root element if `color="error"`. */
    colorError: {
      color: e.palette.error.main
    },
    /* Styles applied to the root element if `color="disabled"`. */
    colorDisabled: {
      color: e.palette.action.disabled
    },
    /* Styles applied to the root element if `fontSize="inherit"`. */
    fontSizeInherit: {
      fontSize: "inherit"
    },
    /* Styles applied to the root element if `fontSize="small"`. */
    fontSizeSmall: {
      fontSize: e.typography.pxToRem(20)
    },
    /* Styles applied to the root element if `fontSize="large"`. */
    fontSizeLarge: {
      fontSize: e.typography.pxToRem(35)
    }
  };
}, nu = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.classes, o = e.className, i = e.color, s = i === void 0 ? "inherit" : i, l = e.component, u = l === void 0 ? "svg" : l, c = e.fontSize, d = c === void 0 ? "medium" : c, p = e.htmlColor, h = e.titleAccess, g = e.viewBox, m = g === void 0 ? "0 0 24 24" : g, y = Ae(e, ["children", "classes", "className", "color", "component", "fontSize", "htmlColor", "titleAccess", "viewBox"]);
  return /* @__PURE__ */ b.createElement(u, Y({
    className: he(n.root, o, s !== "inherit" && n["color".concat(pt(s))], d !== "default" && d !== "medium" && n["fontSize".concat(pt(d))]),
    focusable: "false",
    viewBox: m,
    color: p,
    "aria-hidden": h ? void 0 : !0,
    role: h ? "img" : void 0,
    ref: a
  }, y), r, h ? /* @__PURE__ */ b.createElement("title", null, h) : null);
});
nu.muiName = "SvgIcon";
const hl = et(Bm, {
  name: "MuiSvgIcon"
})(nu);
function Jt(t, e) {
  var a = function(n, o) {
    return /* @__PURE__ */ ht.createElement(hl, Y({
      ref: o
    }, n), t);
  };
  return a.muiName = hl.muiName, /* @__PURE__ */ ht.memo(/* @__PURE__ */ ht.forwardRef(a));
}
function Yn(t) {
  var e = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : 166, a;
  function r() {
    for (var n = arguments.length, o = new Array(n), i = 0; i < n; i++)
      o[i] = arguments[i];
    var s = this, l = function() {
      t.apply(s, o);
    };
    clearTimeout(a), a = setTimeout(l, e);
  }
  return r.clear = function() {
    clearTimeout(a);
  }, r;
}
function Ta(t, e) {
  return /* @__PURE__ */ b.isValidElement(t) && e.indexOf(t.type.muiName) !== -1;
}
function cr(t) {
  return t && t.ownerDocument || document;
}
function rs(t) {
  var e = cr(t);
  return e.defaultView || window;
}
function Hr(t, e) {
  typeof t == "function" ? t(e) : t && (t.current = e);
}
function ns(t) {
  var e = t.controlled, a = t.default;
  t.name, t.state;
  var r = b.useRef(e !== void 0), n = r.current, o = b.useState(a), i = o[0], s = o[1], l = n ? e : i, u = b.useCallback(function(c) {
    n || s(c);
  }, []);
  return [l, u];
}
var Fm = typeof window < "u" ? b.useLayoutEffect : b.useEffect;
function rr(t) {
  var e = b.useRef(t);
  return Fm(function() {
    e.current = t;
  }), b.useCallback(function() {
    return e.current.apply(void 0, arguments);
  }, []);
}
function St(t, e) {
  return b.useMemo(function() {
    return t == null && e == null ? null : function(a) {
      Hr(t, a), Hr(e, a);
    };
  }, [t, e]);
}
function zm(t) {
  var e = b.useState(t), a = e[0], r = e[1], n = t || a;
  return b.useEffect(function() {
    a == null && r("mui-".concat(Math.round(Math.random() * 1e5)));
  }, [a]), n;
}
var eo = !0, Ti = !1, ml = null, Wm = {
  text: !0,
  search: !0,
  url: !0,
  tel: !0,
  email: !0,
  password: !0,
  number: !0,
  date: !0,
  month: !0,
  week: !0,
  time: !0,
  datetime: !0,
  "datetime-local": !0
};
function Vm(t) {
  var e = t.type, a = t.tagName;
  return !!(a === "INPUT" && Wm[e] && !t.readOnly || a === "TEXTAREA" && !t.readOnly || t.isContentEditable);
}
function Hm(t) {
  t.metaKey || t.altKey || t.ctrlKey || (eo = !0);
}
function Xo() {
  eo = !1;
}
function Km() {
  this.visibilityState === "hidden" && Ti && (eo = !0);
}
function Um(t) {
  t.addEventListener("keydown", Hm, !0), t.addEventListener("mousedown", Xo, !0), t.addEventListener("pointerdown", Xo, !0), t.addEventListener("touchstart", Xo, !0), t.addEventListener("visibilitychange", Km, !0);
}
function qm(t) {
  var e = t.target;
  try {
    return e.matches(":focus-visible");
  } catch {
  }
  return eo || Vm(e);
}
function Gm() {
  Ti = !0, window.clearTimeout(ml), ml = window.setTimeout(function() {
    Ti = !1;
  }, 100);
}
function as() {
  var t = b.useCallback(function(e) {
    var a = Ut.findDOMNode(e);
    a != null && Um(a.ownerDocument);
  }, []);
  return {
    isFocusVisible: qm,
    onBlurVisible: Gm,
    ref: t
  };
}
function Ym(t) {
  return Mc(t) || Ic(t) || Ui(t) || Nc();
}
const gl = {
  disabled: !1
}, Wa = ht.createContext(null);
var Xm = function(e) {
  return e.scrollTop;
}, Bn = "unmounted", Mr = "exited", Nr = "entering", pn = "entered", Oi = "exiting", or = /* @__PURE__ */ (function(t) {
  Ja(e, t);
  function e(r, n) {
    var o;
    o = t.call(this, r, n) || this;
    var i = n, s = i && !i.isMounting ? r.enter : r.appear, l;
    return o.appearStatus = null, r.in ? s ? (l = Mr, o.appearStatus = Nr) : l = pn : r.unmountOnExit || r.mountOnEnter ? l = Bn : l = Mr, o.state = {
      status: l
    }, o.nextCallback = null, o;
  }
  e.getDerivedStateFromProps = function(n, o) {
    var i = n.in;
    return i && o.status === Bn ? {
      status: Mr
    } : null;
  };
  var a = e.prototype;
  return a.componentDidMount = function() {
    this.updateStatus(!0, this.appearStatus);
  }, a.componentDidUpdate = function(n) {
    var o = null;
    if (n !== this.props) {
      var i = this.state.status;
      this.props.in ? i !== Nr && i !== pn && (o = Nr) : (i === Nr || i === pn) && (o = Oi);
    }
    this.updateStatus(!1, o);
  }, a.componentWillUnmount = function() {
    this.cancelNextCallback();
  }, a.getTimeouts = function() {
    var n = this.props.timeout, o, i, s;
    return o = i = s = n, n != null && typeof n != "number" && (o = n.exit, i = n.enter, s = n.appear !== void 0 ? n.appear : i), {
      exit: o,
      enter: i,
      appear: s
    };
  }, a.updateStatus = function(n, o) {
    if (n === void 0 && (n = !1), o !== null)
      if (this.cancelNextCallback(), o === Nr) {
        if (this.props.unmountOnExit || this.props.mountOnEnter) {
          var i = this.props.nodeRef ? this.props.nodeRef.current : ga.findDOMNode(this);
          i && Xm(i);
        }
        this.performEnter(n);
      } else
        this.performExit();
    else this.props.unmountOnExit && this.state.status === Mr && this.setState({
      status: Bn
    });
  }, a.performEnter = function(n) {
    var o = this, i = this.props.enter, s = this.context ? this.context.isMounting : n, l = this.props.nodeRef ? [s] : [ga.findDOMNode(this), s], u = l[0], c = l[1], d = this.getTimeouts(), p = s ? d.appear : d.enter;
    if (!n && !i || gl.disabled) {
      this.safeSetState({
        status: pn
      }, function() {
        o.props.onEntered(u);
      });
      return;
    }
    this.props.onEnter(u, c), this.safeSetState({
      status: Nr
    }, function() {
      o.props.onEntering(u, c), o.onTransitionEnd(p, function() {
        o.safeSetState({
          status: pn
        }, function() {
          o.props.onEntered(u, c);
        });
      });
    });
  }, a.performExit = function() {
    var n = this, o = this.props.exit, i = this.getTimeouts(), s = this.props.nodeRef ? void 0 : ga.findDOMNode(this);
    if (!o || gl.disabled) {
      this.safeSetState({
        status: Mr
      }, function() {
        n.props.onExited(s);
      });
      return;
    }
    this.props.onExit(s), this.safeSetState({
      status: Oi
    }, function() {
      n.props.onExiting(s), n.onTransitionEnd(i.exit, function() {
        n.safeSetState({
          status: Mr
        }, function() {
          n.props.onExited(s);
        });
      });
    });
  }, a.cancelNextCallback = function() {
    this.nextCallback !== null && (this.nextCallback.cancel(), this.nextCallback = null);
  }, a.safeSetState = function(n, o) {
    o = this.setNextCallback(o), this.setState(n, o);
  }, a.setNextCallback = function(n) {
    var o = this, i = !0;
    return this.nextCallback = function(s) {
      i && (i = !1, o.nextCallback = null, n(s));
    }, this.nextCallback.cancel = function() {
      i = !1;
    }, this.nextCallback;
  }, a.onTransitionEnd = function(n, o) {
    this.setNextCallback(o);
    var i = this.props.nodeRef ? this.props.nodeRef.current : ga.findDOMNode(this), s = n == null && !this.props.addEndListener;
    if (!i || s) {
      setTimeout(this.nextCallback, 0);
      return;
    }
    if (this.props.addEndListener) {
      var l = this.props.nodeRef ? [this.nextCallback] : [i, this.nextCallback], u = l[0], c = l[1];
      this.props.addEndListener(u, c);
    }
    n != null && setTimeout(this.nextCallback, n);
  }, a.render = function() {
    var n = this.state.status;
    if (n === Bn)
      return null;
    var o = this.props, i = o.children;
    o.in, o.mountOnEnter, o.unmountOnExit, o.appear, o.enter, o.exit, o.timeout, o.addEndListener, o.onEnter, o.onEntering, o.onEntered, o.onExit, o.onExiting, o.onExited, o.nodeRef;
    var s = Ya(o, ["children", "in", "mountOnEnter", "unmountOnExit", "appear", "enter", "exit", "timeout", "addEndListener", "onEnter", "onEntering", "onEntered", "onExit", "onExiting", "onExited", "nodeRef"]);
    return (
      // allows for nested Transitions
      /* @__PURE__ */ ht.createElement(Wa.Provider, {
        value: null
      }, typeof i == "function" ? i(n, s) : ht.cloneElement(ht.Children.only(i), s))
    );
  }, e;
})(ht.Component);
or.contextType = Wa;
or.propTypes = {};
function on() {
}
or.defaultProps = {
  in: !1,
  mountOnEnter: !1,
  unmountOnExit: !1,
  appear: !1,
  enter: !0,
  exit: !0,
  onEnter: on,
  onEntering: on,
  onEntered: on,
  onExit: on,
  onExiting: on,
  onExited: on
};
or.UNMOUNTED = Bn;
or.EXITED = Mr;
or.ENTERING = Nr;
or.ENTERED = pn;
or.EXITING = Oi;
function os(t, e) {
  var a = function(o) {
    return e && $a(o) ? e(o) : o;
  }, r = /* @__PURE__ */ Object.create(null);
  return t && Id.map(t, function(n) {
    return n;
  }).forEach(function(n) {
    r[n.key] = a(n);
  }), r;
}
function Jm(t, e) {
  t = t || {}, e = e || {};
  function a(c) {
    return c in e ? e[c] : t[c];
  }
  var r = /* @__PURE__ */ Object.create(null), n = [];
  for (var o in t)
    o in e ? n.length && (r[o] = n, n = []) : n.push(o);
  var i, s = {};
  for (var l in e) {
    if (r[l])
      for (i = 0; i < r[l].length; i++) {
        var u = r[l][i];
        s[r[l][i]] = a(u);
      }
    s[l] = a(l);
  }
  for (i = 0; i < n.length; i++)
    s[n[i]] = a(n[i]);
  return s;
}
function Dr(t, e, a) {
  return a[e] != null ? a[e] : t.props[e];
}
function Zm(t, e) {
  return os(t.children, function(a) {
    return Ra(a, {
      onExited: e.bind(null, a),
      in: !0,
      appear: Dr(a, "appear", t),
      enter: Dr(a, "enter", t),
      exit: Dr(a, "exit", t)
    });
  });
}
function Qm(t, e, a) {
  var r = os(t.children), n = Jm(e, r);
  return Object.keys(n).forEach(function(o) {
    var i = n[o];
    if ($a(i)) {
      var s = o in e, l = o in r, u = e[o], c = $a(u) && !u.props.in;
      l && (!s || c) ? n[o] = Ra(i, {
        onExited: a.bind(null, i),
        in: !0,
        exit: Dr(i, "exit", t),
        enter: Dr(i, "enter", t)
      }) : !l && s && !c ? n[o] = Ra(i, {
        in: !1
      }) : l && s && $a(u) && (n[o] = Ra(i, {
        onExited: a.bind(null, i),
        in: u.props.in,
        exit: Dr(i, "exit", t),
        enter: Dr(i, "enter", t)
      }));
    }
  }), n;
}
var eg = Object.values || function(t) {
  return Object.keys(t).map(function(e) {
    return t[e];
  });
}, tg = {
  component: "div",
  childFactory: function(e) {
    return e;
  }
}, is = /* @__PURE__ */ (function(t) {
  Ja(e, t);
  function e(r, n) {
    var o;
    o = t.call(this, r, n) || this;
    var i = o.handleExited.bind(bi(o));
    return o.state = {
      contextValue: {
        isMounting: !0
      },
      handleExited: i,
      firstRender: !0
    }, o;
  }
  var a = e.prototype;
  return a.componentDidMount = function() {
    this.mounted = !0, this.setState({
      contextValue: {
        isMounting: !1
      }
    });
  }, a.componentWillUnmount = function() {
    this.mounted = !1;
  }, e.getDerivedStateFromProps = function(n, o) {
    var i = o.children, s = o.handleExited, l = o.firstRender;
    return {
      children: l ? Zm(n, s) : Qm(n, i, s),
      firstRender: !1
    };
  }, a.handleExited = function(n, o) {
    var i = os(this.props.children);
    n.key in i || (n.props.onExited && n.props.onExited(o), this.mounted && this.setState(function(s) {
      var l = Y({}, s.children);
      return delete l[n.key], {
        children: l
      };
    }));
  }, a.render = function() {
    var n = this.props, o = n.component, i = n.childFactory, s = Ya(n, ["component", "childFactory"]), l = this.state.contextValue, u = eg(this.state.children).map(i);
    return delete s.appear, delete s.enter, delete s.exit, o === null ? /* @__PURE__ */ ht.createElement(Wa.Provider, {
      value: l
    }, u) : /* @__PURE__ */ ht.createElement(Wa.Provider, {
      value: l
    }, /* @__PURE__ */ ht.createElement(o, s, u));
  }, e;
})(ht.Component);
is.propTypes = {};
is.defaultProps = tg;
var rg = function(e) {
  return e.scrollTop;
};
function Va(t, e) {
  var a = t.timeout, r = t.style, n = r === void 0 ? {} : r;
  return {
    duration: n.transitionDuration || typeof a == "number" ? a : a[e.mode] || 0,
    delay: n.transitionDelay
  };
}
var ng = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      height: 0,
      overflow: "hidden",
      transition: e.transitions.create("height")
    },
    /* Styles applied to the root element when the transition has entered. */
    entered: {
      height: "auto",
      overflow: "visible"
    },
    /* Styles applied to the root element when the transition has exited and `collapsedSize` != 0px. */
    hidden: {
      visibility: "hidden"
    },
    /* Styles applied to the outer wrapper element. */
    wrapper: {
      // Hack to get children with a negative margin to not falsify the height computation.
      display: "flex"
    },
    /* Styles applied to the inner wrapper element. */
    wrapperInner: {
      width: "100%"
    }
  };
}, au = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.classes, o = e.className, i = e.collapsedHeight, s = e.collapsedSize, l = s === void 0 ? "0px" : s, u = e.component, c = u === void 0 ? "div" : u, d = e.disableStrictModeCompat, p = d === void 0 ? !1 : d, h = e.in, g = e.onEnter, m = e.onEntered, y = e.onEntering, w = e.onExit, k = e.onExited, j = e.onExiting, f = e.style, x = e.timeout, E = x === void 0 ? gi.standard : x, L = e.TransitionComponent, W = L === void 0 ? or : L, O = Ae(e, ["children", "classes", "className", "collapsedHeight", "collapsedSize", "component", "disableStrictModeCompat", "in", "onEnter", "onEntered", "onEntering", "onExit", "onExited", "onExiting", "style", "timeout", "TransitionComponent"]), P = $n(), A = b.useRef(), v = b.useRef(null), C = b.useRef(), M = typeof (i || l) == "number" ? "".concat(i || l, "px") : i || l;
  b.useEffect(function() {
    return function() {
      clearTimeout(A.current);
    };
  }, []);
  var D = P.unstable_strictMode && !p, $ = b.useRef(null), H = St(a, D ? $ : void 0), F = function(ye) {
    return function(be, we) {
      if (ye) {
        var Ee = D ? [$.current, be] : [be, we], ge = En(Ee, 2), ke = ge[0], je = ge[1];
        je === void 0 ? ye(ke) : ye(ke, je);
      }
    };
  }, q = F(function(fe, ye) {
    fe.style.height = M, g && g(fe, ye);
  }), z = F(function(fe, ye) {
    var be = v.current ? v.current.clientHeight : 0, we = Va({
      style: f,
      timeout: E
    }, {
      mode: "enter"
    }), Ee = we.duration;
    if (E === "auto") {
      var ge = P.transitions.getAutoHeightDuration(be);
      fe.style.transitionDuration = "".concat(ge, "ms"), C.current = ge;
    } else
      fe.style.transitionDuration = typeof Ee == "string" ? Ee : "".concat(Ee, "ms");
    fe.style.height = "".concat(be, "px"), y && y(fe, ye);
  }), Z = F(function(fe, ye) {
    fe.style.height = "auto", m && m(fe, ye);
  }), Q = F(function(fe) {
    var ye = v.current ? v.current.clientHeight : 0;
    fe.style.height = "".concat(ye, "px"), w && w(fe);
  }), ue = F(k), de = F(function(fe) {
    var ye = v.current ? v.current.clientHeight : 0, be = Va({
      style: f,
      timeout: E
    }, {
      mode: "exit"
    }), we = be.duration;
    if (E === "auto") {
      var Ee = P.transitions.getAutoHeightDuration(ye);
      fe.style.transitionDuration = "".concat(Ee, "ms"), C.current = Ee;
    } else
      fe.style.transitionDuration = typeof we == "string" ? we : "".concat(we, "ms");
    fe.style.height = M, j && j(fe);
  }), Se = function(ye, be) {
    var we = D ? ye : be;
    E === "auto" && (A.current = setTimeout(we, C.current || 0));
  };
  return /* @__PURE__ */ b.createElement(W, Y({
    in: h,
    onEnter: q,
    onEntered: Z,
    onEntering: z,
    onExit: Q,
    onExited: ue,
    onExiting: de,
    addEndListener: Se,
    nodeRef: D ? $ : void 0,
    timeout: E === "auto" ? null : E
  }, O), function(fe, ye) {
    return /* @__PURE__ */ b.createElement(c, Y({
      className: he(n.root, n.container, o, {
        entered: n.entered,
        exited: !h && M === "0px" && n.hidden
      }[fe]),
      style: Y({
        minHeight: M
      }, f),
      ref: H
    }, ye), /* @__PURE__ */ b.createElement("div", {
      className: n.wrapper,
      ref: v
    }, /* @__PURE__ */ b.createElement("div", {
      className: n.wrapperInner
    }, r)));
  });
});
au.muiSupportAuto = !0;
const ag = et(ng, {
  name: "MuiCollapse"
})(au);
var og = function(e) {
  var a = {};
  return e.shadows.forEach(function(r, n) {
    a["elevation".concat(n)] = {
      boxShadow: r
    };
  }), Y({
    /* Styles applied to the root element. */
    root: {
      backgroundColor: e.palette.background.paper,
      color: e.palette.text.primary,
      transition: e.transitions.create("box-shadow")
    },
    /* Styles applied to the root element if `square={false}`. */
    rounded: {
      borderRadius: e.shape.borderRadius
    },
    /* Styles applied to the root element if `variant="outlined"`. */
    outlined: {
      border: "1px solid ".concat(e.palette.divider)
    }
  }, a);
}, ig = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.classes, n = e.className, o = e.component, i = o === void 0 ? "div" : o, s = e.square, l = s === void 0 ? !1 : s, u = e.elevation, c = u === void 0 ? 1 : u, d = e.variant, p = d === void 0 ? "elevation" : d, h = Ae(e, ["classes", "className", "component", "square", "elevation", "variant"]);
  return /* @__PURE__ */ b.createElement(i, Y({
    className: he(r.root, n, p === "outlined" ? r.outlined : r["elevation".concat(c)], !l && r.rounded),
    ref: a
  }, h));
});
const ou = et(og, {
  name: "MuiPaper"
})(ig);
var iu = b.createContext({}), sg = function(e) {
  var a = {
    duration: e.transitions.duration.shortest
  };
  return {
    /* Styles applied to the root element. */
    root: {
      position: "relative",
      transition: e.transitions.create(["margin"], a),
      "&:before": {
        position: "absolute",
        left: 0,
        top: -1,
        right: 0,
        height: 1,
        content: '""',
        opacity: 1,
        backgroundColor: e.palette.divider,
        transition: e.transitions.create(["opacity", "background-color"], a)
      },
      "&:first-child": {
        "&:before": {
          display: "none"
        }
      },
      "&$expanded": {
        margin: "16px 0",
        "&:first-child": {
          marginTop: 0
        },
        "&:last-child": {
          marginBottom: 0
        },
        "&:before": {
          opacity: 0
        }
      },
      "&$expanded + &": {
        "&:before": {
          display: "none"
        }
      },
      "&$disabled": {
        backgroundColor: e.palette.action.disabledBackground
      }
    },
    /* Styles applied to the root element if `square={false}`. */
    rounded: {
      borderRadius: 0,
      "&:first-child": {
        borderTopLeftRadius: e.shape.borderRadius,
        borderTopRightRadius: e.shape.borderRadius
      },
      "&:last-child": {
        borderBottomLeftRadius: e.shape.borderRadius,
        borderBottomRightRadius: e.shape.borderRadius,
        // Fix a rendering issue on Edge
        "@supports (-ms-ime-align: auto)": {
          borderBottomLeftRadius: 0,
          borderBottomRightRadius: 0
        }
      }
    },
    /* Styles applied to the root element if `expanded={true}`. */
    expanded: {},
    /* Styles applied to the root element if `disabled={true}`. */
    disabled: {}
  };
}, lg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.classes, o = e.className, i = e.defaultExpanded, s = i === void 0 ? !1 : i, l = e.disabled, u = l === void 0 ? !1 : l, c = e.expanded, d = e.onChange, p = e.square, h = p === void 0 ? !1 : p, g = e.TransitionComponent, m = g === void 0 ? ag : g, y = e.TransitionProps, w = Ae(e, ["children", "classes", "className", "defaultExpanded", "disabled", "expanded", "onChange", "square", "TransitionComponent", "TransitionProps"]), k = ns({
    controlled: c,
    default: s,
    name: "Accordion",
    state: "expanded"
  }), j = En(k, 2), f = j[0], x = j[1], E = b.useCallback(function(v) {
    x(!f), d && d(v, !f);
  }, [f, d, x]), L = b.Children.toArray(r), W = Ym(L), O = W[0], P = W.slice(1), A = b.useMemo(function() {
    return {
      expanded: f,
      disabled: u,
      toggle: E
    };
  }, [f, u, E]);
  return /* @__PURE__ */ b.createElement(ou, Y({
    className: he(n.root, o, f && n.expanded, u && n.disabled, !h && n.rounded),
    ref: a,
    square: h
  }, w), /* @__PURE__ */ b.createElement(iu.Provider, {
    value: A
  }, O), /* @__PURE__ */ b.createElement(m, Y({
    in: f,
    timeout: "auto"
  }, y), /* @__PURE__ */ b.createElement("div", {
    "aria-labelledby": O.props.id,
    id: O.props["aria-controls"],
    role: "region"
  }, P)));
});
const cg = et(sg, {
  name: "MuiAccordion"
})(lg);
var ug = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      display: "flex",
      padding: e.spacing(1, 2, 2)
    }
  };
}, dg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.classes, n = e.className, o = Ae(e, ["classes", "className"]);
  return /* @__PURE__ */ b.createElement("div", Y({
    className: he(r.root, n),
    ref: a
  }, o));
});
const fg = et(ug, {
  name: "MuiAccordionDetails"
})(dg);
var pg = typeof window > "u" ? b.useEffect : b.useLayoutEffect;
function hg(t) {
  var e = t.classes, a = t.pulsate, r = a === void 0 ? !1 : a, n = t.rippleX, o = t.rippleY, i = t.rippleSize, s = t.in, l = t.onExited, u = l === void 0 ? function() {
  } : l, c = t.timeout, d = b.useState(!1), p = d[0], h = d[1], g = he(e.ripple, e.rippleVisible, r && e.ripplePulsate), m = {
    width: i,
    height: i,
    top: -(i / 2) + o,
    left: -(i / 2) + n
  }, y = he(e.child, p && e.childLeaving, r && e.childPulsate), w = rr(u);
  return pg(function() {
    if (!s) {
      h(!0);
      var k = setTimeout(w, c);
      return function() {
        clearTimeout(k);
      };
    }
  }, [w, s, c]), /* @__PURE__ */ b.createElement("span", {
    className: g,
    style: m
  }, /* @__PURE__ */ b.createElement("span", {
    className: y
  }));
}
var _i = 550, mg = 80, gg = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      overflow: "hidden",
      pointerEvents: "none",
      position: "absolute",
      zIndex: 0,
      top: 0,
      right: 0,
      bottom: 0,
      left: 0,
      borderRadius: "inherit"
    },
    /* Styles applied to the internal `Ripple` components `ripple` class. */
    ripple: {
      opacity: 0,
      position: "absolute"
    },
    /* Styles applied to the internal `Ripple` components `rippleVisible` class. */
    rippleVisible: {
      opacity: 0.3,
      transform: "scale(1)",
      animation: "$enter ".concat(_i, "ms ").concat(e.transitions.easing.easeInOut)
    },
    /* Styles applied to the internal `Ripple` components `ripplePulsate` class. */
    ripplePulsate: {
      animationDuration: "".concat(e.transitions.duration.shorter, "ms")
    },
    /* Styles applied to the internal `Ripple` components `child` class. */
    child: {
      opacity: 1,
      display: "block",
      width: "100%",
      height: "100%",
      borderRadius: "50%",
      backgroundColor: "currentColor"
    },
    /* Styles applied to the internal `Ripple` components `childLeaving` class. */
    childLeaving: {
      opacity: 0,
      animation: "$exit ".concat(_i, "ms ").concat(e.transitions.easing.easeInOut)
    },
    /* Styles applied to the internal `Ripple` components `childPulsate` class. */
    childPulsate: {
      position: "absolute",
      left: 0,
      top: 0,
      animation: "$pulsate 2500ms ".concat(e.transitions.easing.easeInOut, " 200ms infinite")
    },
    "@keyframes enter": {
      "0%": {
        transform: "scale(0)",
        opacity: 0.1
      },
      "100%": {
        transform: "scale(1)",
        opacity: 0.3
      }
    },
    "@keyframes exit": {
      "0%": {
        opacity: 1
      },
      "100%": {
        opacity: 0
      }
    },
    "@keyframes pulsate": {
      "0%": {
        transform: "scale(1)"
      },
      "50%": {
        transform: "scale(0.92)"
      },
      "100%": {
        transform: "scale(1)"
      }
    }
  };
}, vg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.center, n = r === void 0 ? !1 : r, o = e.classes, i = e.className, s = Ae(e, ["center", "classes", "className"]), l = b.useState([]), u = l[0], c = l[1], d = b.useRef(0), p = b.useRef(null);
  b.useEffect(function() {
    p.current && (p.current(), p.current = null);
  }, [u]);
  var h = b.useRef(!1), g = b.useRef(null), m = b.useRef(null), y = b.useRef(null);
  b.useEffect(function() {
    return function() {
      clearTimeout(g.current);
    };
  }, []);
  var w = b.useCallback(function(x) {
    var E = x.pulsate, L = x.rippleX, W = x.rippleY, O = x.rippleSize, P = x.cb;
    c(function(A) {
      return [].concat(Xa(A), [/* @__PURE__ */ b.createElement(hg, {
        key: d.current,
        classes: o,
        timeout: _i,
        pulsate: E,
        rippleX: L,
        rippleY: W,
        rippleSize: O
      })]);
    }), d.current += 1, p.current = P;
  }, [o]), k = b.useCallback(function() {
    var x = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : {}, E = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {}, L = arguments.length > 2 ? arguments[2] : void 0, W = E.pulsate, O = W === void 0 ? !1 : W, P = E.center, A = P === void 0 ? n || E.pulsate : P, v = E.fakeElement, C = v === void 0 ? !1 : v;
    if (x.type === "mousedown" && h.current) {
      h.current = !1;
      return;
    }
    x.type === "touchstart" && (h.current = !0);
    var M = C ? null : y.current, D = M ? M.getBoundingClientRect() : {
      width: 0,
      height: 0,
      left: 0,
      top: 0
    }, $, H, F;
    if (A || x.clientX === 0 && x.clientY === 0 || !x.clientX && !x.touches)
      $ = Math.round(D.width / 2), H = Math.round(D.height / 2);
    else {
      var q = x.touches ? x.touches[0] : x, z = q.clientX, Z = q.clientY;
      $ = Math.round(z - D.left), H = Math.round(Z - D.top);
    }
    if (A)
      F = Math.sqrt((2 * Math.pow(D.width, 2) + Math.pow(D.height, 2)) / 3), F % 2 === 0 && (F += 1);
    else {
      var Q = Math.max(Math.abs((M ? M.clientWidth : 0) - $), $) * 2 + 2, ue = Math.max(Math.abs((M ? M.clientHeight : 0) - H), H) * 2 + 2;
      F = Math.sqrt(Math.pow(Q, 2) + Math.pow(ue, 2));
    }
    x.touches ? m.current === null && (m.current = function() {
      w({
        pulsate: O,
        rippleX: $,
        rippleY: H,
        rippleSize: F,
        cb: L
      });
    }, g.current = setTimeout(function() {
      m.current && (m.current(), m.current = null);
    }, mg)) : w({
      pulsate: O,
      rippleX: $,
      rippleY: H,
      rippleSize: F,
      cb: L
    });
  }, [n, w]), j = b.useCallback(function() {
    k({}, {
      pulsate: !0
    });
  }, [k]), f = b.useCallback(function(x, E) {
    if (clearTimeout(g.current), x.type === "touchend" && m.current) {
      x.persist(), m.current(), m.current = null, g.current = setTimeout(function() {
        f(x, E);
      });
      return;
    }
    m.current = null, c(function(L) {
      return L.length > 0 ? L.slice(1) : L;
    }), p.current = E;
  }, []);
  return b.useImperativeHandle(a, function() {
    return {
      pulsate: j,
      start: k,
      stop: f
    };
  }, [j, k, f]), /* @__PURE__ */ b.createElement("span", Y({
    className: he(o.root, i),
    ref: y
  }, s), /* @__PURE__ */ b.createElement(is, {
    component: null,
    exit: !0
  }, u));
});
const bg = et(gg, {
  flip: !1,
  name: "MuiTouchRipple"
})(/* @__PURE__ */ b.memo(vg));
var yg = {
  /* Styles applied to the root element. */
  root: {
    display: "inline-flex",
    alignItems: "center",
    justifyContent: "center",
    position: "relative",
    WebkitTapHighlightColor: "transparent",
    backgroundColor: "transparent",
    // Reset default value
    // We disable the focus ring for mouse, touch and keyboard users.
    outline: 0,
    border: 0,
    margin: 0,
    // Remove the margin in Safari
    borderRadius: 0,
    padding: 0,
    // Remove the padding in Firefox
    cursor: "pointer",
    userSelect: "none",
    verticalAlign: "middle",
    "-moz-appearance": "none",
    // Reset
    "-webkit-appearance": "none",
    // Reset
    textDecoration: "none",
    // So we take precedent over the style of a native <a /> element.
    color: "inherit",
    "&::-moz-focus-inner": {
      borderStyle: "none"
      // Remove Firefox dotted outline.
    },
    "&$disabled": {
      pointerEvents: "none",
      // Disable link interactions
      cursor: "default"
    },
    "@media print": {
      colorAdjust: "exact"
    }
  },
  /* Pseudo-class applied to the root element if `disabled={true}`. */
  disabled: {},
  /* Pseudo-class applied to the root element if keyboard focused. */
  focusVisible: {}
}, xg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.action, n = e.buttonRef, o = e.centerRipple, i = o === void 0 ? !1 : o, s = e.children, l = e.classes, u = e.className, c = e.component, d = c === void 0 ? "button" : c, p = e.disabled, h = p === void 0 ? !1 : p, g = e.disableRipple, m = g === void 0 ? !1 : g, y = e.disableTouchRipple, w = y === void 0 ? !1 : y, k = e.focusRipple, j = k === void 0 ? !1 : k, f = e.focusVisibleClassName, x = e.onBlur, E = e.onClick, L = e.onFocus, W = e.onFocusVisible, O = e.onKeyDown, P = e.onKeyUp, A = e.onMouseDown, v = e.onMouseLeave, C = e.onMouseUp, M = e.onTouchEnd, D = e.onTouchMove, $ = e.onTouchStart, H = e.onDragLeave, F = e.tabIndex, q = F === void 0 ? 0 : F, z = e.TouchRippleProps, Z = e.type, Q = Z === void 0 ? "button" : Z, ue = Ae(e, ["action", "buttonRef", "centerRipple", "children", "classes", "className", "component", "disabled", "disableRipple", "disableTouchRipple", "focusRipple", "focusVisibleClassName", "onBlur", "onClick", "onFocus", "onFocusVisible", "onKeyDown", "onKeyUp", "onMouseDown", "onMouseLeave", "onMouseUp", "onTouchEnd", "onTouchMove", "onTouchStart", "onDragLeave", "tabIndex", "TouchRippleProps", "type"]), de = b.useRef(null);
  function Se() {
    return Ut.findDOMNode(de.current);
  }
  var fe = b.useRef(null), ye = b.useState(!1), be = ye[0], we = ye[1];
  h && be && we(!1);
  var Ee = as(), ge = Ee.isFocusVisible, ke = Ee.onBlurVisible, je = Ee.ref;
  b.useImperativeHandle(r, function() {
    return {
      focusVisible: function() {
        we(!0), de.current.focus();
      }
    };
  }, []), b.useEffect(function() {
    be && j && !m && fe.current.pulsate();
  }, [m, j, be]);
  function Te(ie, $e) {
    var qe = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : w;
    return rr(function(Je) {
      $e && $e(Je);
      var at = qe;
      return !at && fe.current && fe.current[ie](Je), !0;
    });
  }
  var He = Te("start", A), _e = Te("stop", H), Fe = Te("stop", C), We = Te("stop", function(ie) {
    be && ie.preventDefault(), v && v(ie);
  }), Ue = Te("start", $), tt = Te("stop", M), Qe = Te("stop", D), Ve = Te("stop", function(ie) {
    be && (ke(ie), we(!1)), x && x(ie);
  }, !1), ce = rr(function(ie) {
    de.current || (de.current = ie.currentTarget), ge(ie) && (we(!0), W && W(ie)), L && L(ie);
  }), pe = function() {
    var $e = Se();
    return d && d !== "button" && !($e.tagName === "A" && $e.href);
  }, X = b.useRef(!1), V = rr(function(ie) {
    j && !X.current && be && fe.current && ie.key === " " && (X.current = !0, ie.persist(), fe.current.stop(ie, function() {
      fe.current.start(ie);
    })), ie.target === ie.currentTarget && pe() && ie.key === " " && ie.preventDefault(), O && O(ie), ie.target === ie.currentTarget && pe() && ie.key === "Enter" && !h && (ie.preventDefault(), E && E(ie));
  }), U = rr(function(ie) {
    j && ie.key === " " && fe.current && be && !ie.defaultPrevented && (X.current = !1, ie.persist(), fe.current.stop(ie, function() {
      fe.current.pulsate(ie);
    })), P && P(ie), E && ie.target === ie.currentTarget && pe() && ie.key === " " && !ie.defaultPrevented && E(ie);
  }), te = d;
  te === "button" && ue.href && (te = "a");
  var oe = {};
  te === "button" ? (oe.type = Q, oe.disabled = h) : ((te !== "a" || !ue.href) && (oe.role = "button"), oe["aria-disabled"] = h);
  var xe = St(n, a), Me = St(je, de), Ce = St(xe, Me), me = b.useState(!1), De = me[0], ze = me[1];
  b.useEffect(function() {
    ze(!0);
  }, []);
  var Pe = De && !m && !h;
  return /* @__PURE__ */ b.createElement(te, Y({
    className: he(l.root, u, be && [l.focusVisible, f], h && l.disabled),
    onBlur: Ve,
    onClick: E,
    onFocus: ce,
    onKeyDown: V,
    onKeyUp: U,
    onMouseDown: He,
    onMouseLeave: We,
    onMouseUp: Fe,
    onDragLeave: _e,
    onTouchEnd: tt,
    onTouchMove: Qe,
    onTouchStart: Ue,
    ref: Ce,
    tabIndex: h ? -1 : q
  }, oe, ue), s, Pe ? (
    /* TouchRipple is only needed client-side, x2 boost on the server. */
    /* @__PURE__ */ b.createElement(bg, Y({
      ref: fe,
      center: i
    }, z))
  ) : null);
});
const kr = et(yg, {
  name: "MuiButtonBase"
})(xg);
var Cg = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      textAlign: "center",
      flex: "0 0 auto",
      fontSize: e.typography.pxToRem(24),
      padding: 12,
      borderRadius: "50%",
      overflow: "visible",
      // Explicitly set the default value to solve a bug on IE 11.
      color: e.palette.action.active,
      transition: e.transitions.create("background-color", {
        duration: e.transitions.duration.shortest
      }),
      "&:hover": {
        backgroundColor: Ye(e.palette.action.active, e.palette.action.hoverOpacity),
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: "transparent"
        }
      },
      "&$disabled": {
        backgroundColor: "transparent",
        color: e.palette.action.disabled
      }
    },
    /* Styles applied to the root element if `edge="start"`. */
    edgeStart: {
      marginLeft: -12,
      "$sizeSmall&": {
        marginLeft: -3
      }
    },
    /* Styles applied to the root element if `edge="end"`. */
    edgeEnd: {
      marginRight: -12,
      "$sizeSmall&": {
        marginRight: -3
      }
    },
    /* Styles applied to the root element if `color="inherit"`. */
    colorInherit: {
      color: "inherit"
    },
    /* Styles applied to the root element if `color="primary"`. */
    colorPrimary: {
      color: e.palette.primary.main,
      "&:hover": {
        backgroundColor: Ye(e.palette.primary.main, e.palette.action.hoverOpacity),
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: "transparent"
        }
      }
    },
    /* Styles applied to the root element if `color="secondary"`. */
    colorSecondary: {
      color: e.palette.secondary.main,
      "&:hover": {
        backgroundColor: Ye(e.palette.secondary.main, e.palette.action.hoverOpacity),
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: "transparent"
        }
      }
    },
    /* Pseudo-class applied to the root element if `disabled={true}`. */
    disabled: {},
    /* Styles applied to the root element if `size="small"`. */
    sizeSmall: {
      padding: 3,
      fontSize: e.typography.pxToRem(18)
    },
    /* Styles applied to the children container element. */
    label: {
      width: "100%",
      display: "flex",
      alignItems: "inherit",
      justifyContent: "inherit"
    }
  };
}, Sg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.edge, n = r === void 0 ? !1 : r, o = e.children, i = e.classes, s = e.className, l = e.color, u = l === void 0 ? "default" : l, c = e.disabled, d = c === void 0 ? !1 : c, p = e.disableFocusRipple, h = p === void 0 ? !1 : p, g = e.size, m = g === void 0 ? "medium" : g, y = Ae(e, ["edge", "children", "classes", "className", "color", "disabled", "disableFocusRipple", "size"]);
  return /* @__PURE__ */ b.createElement(kr, Y({
    className: he(i.root, s, u !== "default" && i["color".concat(pt(u))], d && i.disabled, m === "small" && i["size".concat(pt(m))], {
      start: i.edgeStart,
      end: i.edgeEnd
    }[n]),
    centerRipple: !0,
    focusRipple: !h,
    disabled: d,
    ref: a
  }, y), /* @__PURE__ */ b.createElement("span", {
    className: i.label
  }, o));
});
const su = et(Cg, {
  name: "MuiIconButton"
})(Sg);
var wg = function(e) {
  var a = {
    duration: e.transitions.duration.shortest
  };
  return {
    /* Styles applied to the root element. */
    root: {
      display: "flex",
      minHeight: 48,
      transition: e.transitions.create(["min-height", "background-color"], a),
      padding: e.spacing(0, 2),
      "&:hover:not($disabled)": {
        cursor: "pointer"
      },
      "&$expanded": {
        minHeight: 64
      },
      "&$focused, &$focusVisible": {
        backgroundColor: e.palette.action.focus
      },
      "&$disabled": {
        opacity: e.palette.action.disabledOpacity
      }
    },
    /* Pseudo-class applied to the root element, children wrapper element and `IconButton` component if `expanded={true}`. */
    expanded: {},
    /* Pseudo-class applied to the ButtonBase root element if the button is keyboard focused. */
    focused: {},
    /* Pseudo-class applied to the ButtonBase root element if the button is keyboard focused. */
    focusVisible: {},
    /* Pseudo-class applied to the root element if `disabled={true}`. */
    disabled: {},
    /* Styles applied to the children wrapper element. */
    content: {
      display: "flex",
      flexGrow: 1,
      transition: e.transitions.create(["margin"], a),
      margin: "12px 0",
      "&$expanded": {
        margin: "20px 0"
      }
    },
    /* Styles applied to the `IconButton` component when `expandIcon` is supplied. */
    expandIcon: {
      transform: "rotate(0deg)",
      transition: e.transitions.create("transform", a),
      "&:hover": {
        // Disable the hover effect for the IconButton,
        // because a hover effect should apply to the entire Expand button and
        // not only to the IconButton.
        backgroundColor: "transparent"
      },
      "&$expanded": {
        transform: "rotate(180deg)"
      }
    }
  };
}, Eg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.classes, o = e.className, i = e.expandIcon, s = e.focusVisibleClassName, l = e.IconButtonProps, u = l === void 0 ? {} : l, c = e.onClick, d = Ae(e, ["children", "classes", "className", "expandIcon", "focusVisibleClassName", "IconButtonProps", "onClick"]), p = b.useContext(iu), h = p.disabled, g = h === void 0 ? !1 : h, m = p.expanded, y = p.toggle, w = function(j) {
    y && y(j), c && c(j);
  };
  return /* @__PURE__ */ b.createElement(kr, Y({
    focusRipple: !1,
    disableRipple: !0,
    disabled: g,
    component: "div",
    "aria-expanded": m,
    className: he(n.root, o, g && n.disabled, m && n.expanded),
    focusVisibleClassName: he(n.focusVisible, n.focused, s),
    onClick: w,
    ref: a
  }, d), /* @__PURE__ */ b.createElement("div", {
    className: he(n.content, m && n.expanded)
  }, r), i && /* @__PURE__ */ b.createElement(su, Y({
    className: he(n.expandIcon, m && n.expanded),
    edge: "end",
    component: "div",
    tabIndex: null,
    role: null,
    "aria-hidden": !0
  }, u), i));
});
const kg = et(wg, {
  name: "MuiAccordionSummary"
})(Eg);
var $g = mf(dr(hf, Sf, Nf, Yf, op, Zf, ip, hp, qi, Op)), ve = ts("div")($g, {
  name: "MuiBox"
}), Rg = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      margin: 0
    },
    /* Styles applied to the root element if `variant="body2"`. */
    body2: e.typography.body2,
    /* Styles applied to the root element if `variant="body1"`. */
    body1: e.typography.body1,
    /* Styles applied to the root element if `variant="caption"`. */
    caption: e.typography.caption,
    /* Styles applied to the root element if `variant="button"`. */
    button: e.typography.button,
    /* Styles applied to the root element if `variant="h1"`. */
    h1: e.typography.h1,
    /* Styles applied to the root element if `variant="h2"`. */
    h2: e.typography.h2,
    /* Styles applied to the root element if `variant="h3"`. */
    h3: e.typography.h3,
    /* Styles applied to the root element if `variant="h4"`. */
    h4: e.typography.h4,
    /* Styles applied to the root element if `variant="h5"`. */
    h5: e.typography.h5,
    /* Styles applied to the root element if `variant="h6"`. */
    h6: e.typography.h6,
    /* Styles applied to the root element if `variant="subtitle1"`. */
    subtitle1: e.typography.subtitle1,
    /* Styles applied to the root element if `variant="subtitle2"`. */
    subtitle2: e.typography.subtitle2,
    /* Styles applied to the root element if `variant="overline"`. */
    overline: e.typography.overline,
    /* Styles applied to the root element if `variant="srOnly"`. Only accessible to screen readers. */
    srOnly: {
      position: "absolute",
      height: 1,
      width: 1,
      overflow: "hidden"
    },
    /* Styles applied to the root element if `align="left"`. */
    alignLeft: {
      textAlign: "left"
    },
    /* Styles applied to the root element if `align="center"`. */
    alignCenter: {
      textAlign: "center"
    },
    /* Styles applied to the root element if `align="right"`. */
    alignRight: {
      textAlign: "right"
    },
    /* Styles applied to the root element if `align="justify"`. */
    alignJustify: {
      textAlign: "justify"
    },
    /* Styles applied to the root element if `nowrap={true}`. */
    noWrap: {
      overflow: "hidden",
      textOverflow: "ellipsis",
      whiteSpace: "nowrap"
    },
    /* Styles applied to the root element if `gutterBottom={true}`. */
    gutterBottom: {
      marginBottom: "0.35em"
    },
    /* Styles applied to the root element if `paragraph={true}`. */
    paragraph: {
      marginBottom: 16
    },
    /* Styles applied to the root element if `color="inherit"`. */
    colorInherit: {
      color: "inherit"
    },
    /* Styles applied to the root element if `color="primary"`. */
    colorPrimary: {
      color: e.palette.primary.main
    },
    /* Styles applied to the root element if `color="secondary"`. */
    colorSecondary: {
      color: e.palette.secondary.main
    },
    /* Styles applied to the root element if `color="textPrimary"`. */
    colorTextPrimary: {
      color: e.palette.text.primary
    },
    /* Styles applied to the root element if `color="textSecondary"`. */
    colorTextSecondary: {
      color: e.palette.text.secondary
    },
    /* Styles applied to the root element if `color="error"`. */
    colorError: {
      color: e.palette.error.main
    },
    /* Styles applied to the root element if `display="inline"`. */
    displayInline: {
      display: "inline"
    },
    /* Styles applied to the root element if `display="block"`. */
    displayBlock: {
      display: "block"
    }
  };
}, vl = {
  h1: "h1",
  h2: "h2",
  h3: "h3",
  h4: "h4",
  h5: "h5",
  h6: "h6",
  subtitle1: "h6",
  subtitle2: "h6",
  body1: "p",
  body2: "p"
}, Tg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.align, n = r === void 0 ? "inherit" : r, o = e.classes, i = e.className, s = e.color, l = s === void 0 ? "initial" : s, u = e.component, c = e.display, d = c === void 0 ? "initial" : c, p = e.gutterBottom, h = p === void 0 ? !1 : p, g = e.noWrap, m = g === void 0 ? !1 : g, y = e.paragraph, w = y === void 0 ? !1 : y, k = e.variant, j = k === void 0 ? "body1" : k, f = e.variantMapping, x = f === void 0 ? vl : f, E = Ae(e, ["align", "classes", "className", "color", "component", "display", "gutterBottom", "noWrap", "paragraph", "variant", "variantMapping"]), L = u || (w ? "p" : x[j] || vl[j]) || "span";
  return /* @__PURE__ */ b.createElement(L, Y({
    className: he(o.root, i, j !== "inherit" && o[j], l !== "initial" && o["color".concat(pt(l))], m && o.noWrap, h && o.gutterBottom, w && o.paragraph, n !== "inherit" && o["align".concat(pt(n))], d !== "initial" && o["display".concat(pt(d))]),
    ref: a
  }, E));
});
const Mt = et(Rg, {
  name: "MuiTypography"
})(Tg);
var Og = function(e) {
  return {
    /* Styles applied to the root element. */
    root: Y({}, e.typography.button, {
      boxSizing: "border-box",
      minWidth: 64,
      padding: "6px 16px",
      borderRadius: e.shape.borderRadius,
      color: e.palette.text.primary,
      transition: e.transitions.create(["background-color", "box-shadow", "border"], {
        duration: e.transitions.duration.short
      }),
      "&:hover": {
        textDecoration: "none",
        backgroundColor: Ye(e.palette.text.primary, e.palette.action.hoverOpacity),
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: "transparent"
        },
        "&$disabled": {
          backgroundColor: "transparent"
        }
      },
      "&$disabled": {
        color: e.palette.action.disabled
      }
    }),
    /* Styles applied to the span element that wraps the children. */
    label: {
      width: "100%",
      // Ensure the correct width for iOS Safari
      display: "inherit",
      alignItems: "inherit",
      justifyContent: "inherit"
    },
    /* Styles applied to the root element if `variant="text"`. */
    text: {
      padding: "6px 8px"
    },
    /* Styles applied to the root element if `variant="text"` and `color="primary"`. */
    textPrimary: {
      color: e.palette.primary.main,
      "&:hover": {
        backgroundColor: Ye(e.palette.primary.main, e.palette.action.hoverOpacity),
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: "transparent"
        }
      }
    },
    /* Styles applied to the root element if `variant="text"` and `color="secondary"`. */
    textSecondary: {
      color: e.palette.secondary.main,
      "&:hover": {
        backgroundColor: Ye(e.palette.secondary.main, e.palette.action.hoverOpacity),
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: "transparent"
        }
      }
    },
    /* Styles applied to the root element if `variant="outlined"`. */
    outlined: {
      padding: "5px 15px",
      border: "1px solid ".concat(e.palette.type === "light" ? "rgba(0, 0, 0, 0.23)" : "rgba(255, 255, 255, 0.23)"),
      "&$disabled": {
        border: "1px solid ".concat(e.palette.action.disabledBackground)
      }
    },
    /* Styles applied to the root element if `variant="outlined"` and `color="primary"`. */
    outlinedPrimary: {
      color: e.palette.primary.main,
      border: "1px solid ".concat(Ye(e.palette.primary.main, 0.5)),
      "&:hover": {
        border: "1px solid ".concat(e.palette.primary.main),
        backgroundColor: Ye(e.palette.primary.main, e.palette.action.hoverOpacity),
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: "transparent"
        }
      }
    },
    /* Styles applied to the root element if `variant="outlined"` and `color="secondary"`. */
    outlinedSecondary: {
      color: e.palette.secondary.main,
      border: "1px solid ".concat(Ye(e.palette.secondary.main, 0.5)),
      "&:hover": {
        border: "1px solid ".concat(e.palette.secondary.main),
        backgroundColor: Ye(e.palette.secondary.main, e.palette.action.hoverOpacity),
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: "transparent"
        }
      },
      "&$disabled": {
        border: "1px solid ".concat(e.palette.action.disabled)
      }
    },
    /* Styles applied to the root element if `variant="contained"`. */
    contained: {
      color: e.palette.getContrastText(e.palette.grey[300]),
      backgroundColor: e.palette.grey[300],
      boxShadow: e.shadows[2],
      "&:hover": {
        backgroundColor: e.palette.grey.A100,
        boxShadow: e.shadows[4],
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          boxShadow: e.shadows[2],
          backgroundColor: e.palette.grey[300]
        },
        "&$disabled": {
          backgroundColor: e.palette.action.disabledBackground
        }
      },
      "&$focusVisible": {
        boxShadow: e.shadows[6]
      },
      "&:active": {
        boxShadow: e.shadows[8]
      },
      "&$disabled": {
        color: e.palette.action.disabled,
        boxShadow: e.shadows[0],
        backgroundColor: e.palette.action.disabledBackground
      }
    },
    /* Styles applied to the root element if `variant="contained"` and `color="primary"`. */
    containedPrimary: {
      color: e.palette.primary.contrastText,
      backgroundColor: e.palette.primary.main,
      "&:hover": {
        backgroundColor: e.palette.primary.dark,
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: e.palette.primary.main
        }
      }
    },
    /* Styles applied to the root element if `variant="contained"` and `color="secondary"`. */
    containedSecondary: {
      color: e.palette.secondary.contrastText,
      backgroundColor: e.palette.secondary.main,
      "&:hover": {
        backgroundColor: e.palette.secondary.dark,
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: e.palette.secondary.main
        }
      }
    },
    /* Styles applied to the root element if `disableElevation={true}`. */
    disableElevation: {
      boxShadow: "none",
      "&:hover": {
        boxShadow: "none"
      },
      "&$focusVisible": {
        boxShadow: "none"
      },
      "&:active": {
        boxShadow: "none"
      },
      "&$disabled": {
        boxShadow: "none"
      }
    },
    /* Pseudo-class applied to the ButtonBase root element if the button is keyboard focused. */
    focusVisible: {},
    /* Pseudo-class applied to the root element if `disabled={true}`. */
    disabled: {},
    /* Styles applied to the root element if `color="inherit"`. */
    colorInherit: {
      color: "inherit",
      borderColor: "currentColor"
    },
    /* Styles applied to the root element if `size="small"` and `variant="text"`. */
    textSizeSmall: {
      padding: "4px 5px",
      fontSize: e.typography.pxToRem(13)
    },
    /* Styles applied to the root element if `size="large"` and `variant="text"`. */
    textSizeLarge: {
      padding: "8px 11px",
      fontSize: e.typography.pxToRem(15)
    },
    /* Styles applied to the root element if `size="small"` and `variant="outlined"`. */
    outlinedSizeSmall: {
      padding: "3px 9px",
      fontSize: e.typography.pxToRem(13)
    },
    /* Styles applied to the root element if `size="large"` and `variant="outlined"`. */
    outlinedSizeLarge: {
      padding: "7px 21px",
      fontSize: e.typography.pxToRem(15)
    },
    /* Styles applied to the root element if `size="small"` and `variant="contained"`. */
    containedSizeSmall: {
      padding: "4px 10px",
      fontSize: e.typography.pxToRem(13)
    },
    /* Styles applied to the root element if `size="large"` and `variant="contained"`. */
    containedSizeLarge: {
      padding: "8px 22px",
      fontSize: e.typography.pxToRem(15)
    },
    /* Styles applied to the root element if `size="small"`. */
    sizeSmall: {},
    /* Styles applied to the root element if `size="large"`. */
    sizeLarge: {},
    /* Styles applied to the root element if `fullWidth={true}`. */
    fullWidth: {
      width: "100%"
    },
    /* Styles applied to the startIcon element if supplied. */
    startIcon: {
      display: "inherit",
      marginRight: 8,
      marginLeft: -4,
      "&$iconSizeSmall": {
        marginLeft: -2
      }
    },
    /* Styles applied to the endIcon element if supplied. */
    endIcon: {
      display: "inherit",
      marginRight: -4,
      marginLeft: 8,
      "&$iconSizeSmall": {
        marginRight: -2
      }
    },
    /* Styles applied to the icon element if supplied and `size="small"`. */
    iconSizeSmall: {
      "& > *:first-child": {
        fontSize: 18
      }
    },
    /* Styles applied to the icon element if supplied and `size="medium"`. */
    iconSizeMedium: {
      "& > *:first-child": {
        fontSize: 20
      }
    },
    /* Styles applied to the icon element if supplied and `size="large"`. */
    iconSizeLarge: {
      "& > *:first-child": {
        fontSize: 22
      }
    }
  };
}, _g = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.classes, o = e.className, i = e.color, s = i === void 0 ? "default" : i, l = e.component, u = l === void 0 ? "button" : l, c = e.disabled, d = c === void 0 ? !1 : c, p = e.disableElevation, h = p === void 0 ? !1 : p, g = e.disableFocusRipple, m = g === void 0 ? !1 : g, y = e.endIcon, w = e.focusVisibleClassName, k = e.fullWidth, j = k === void 0 ? !1 : k, f = e.size, x = f === void 0 ? "medium" : f, E = e.startIcon, L = e.type, W = L === void 0 ? "button" : L, O = e.variant, P = O === void 0 ? "text" : O, A = Ae(e, ["children", "classes", "className", "color", "component", "disabled", "disableElevation", "disableFocusRipple", "endIcon", "focusVisibleClassName", "fullWidth", "size", "startIcon", "type", "variant"]), v = E && /* @__PURE__ */ b.createElement("span", {
    className: he(n.startIcon, n["iconSize".concat(pt(x))])
  }, E), C = y && /* @__PURE__ */ b.createElement("span", {
    className: he(n.endIcon, n["iconSize".concat(pt(x))])
  }, y);
  return /* @__PURE__ */ b.createElement(kr, Y({
    className: he(n.root, n[P], o, s === "inherit" ? n.colorInherit : s !== "default" && n["".concat(P).concat(pt(s))], x !== "medium" && [n["".concat(P, "Size").concat(pt(x))], n["size".concat(pt(x))]], h && n.disableElevation, d && n.disabled, j && n.fullWidth),
    component: u,
    disabled: d,
    focusRipple: !m,
    focusVisibleClassName: he(n.focusVisible, w),
    ref: a,
    type: W
  }, A), /* @__PURE__ */ b.createElement("span", {
    className: n.label
  }, v, r, C));
});
const Pg = et(Og, {
  name: "MuiButton"
})(_g);
var aa = b.createContext();
function lu() {
  return b.useContext(aa);
}
function ss() {
  return b.useContext(aa);
}
const jg = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M12 2C6.47 2 2 6.47 2 12s4.47 10 10 10 10-4.47 10-10S17.53 2 12 2zm5 13.59L15.59 17 12 13.41 8.41 17 7 15.59 10.59 12 7 8.41 8.41 7 12 10.59 15.59 7 17 8.41 13.41 12 17 15.59z"
}));
var Ig = function(e) {
  var a = e.palette.type === "light" ? e.palette.grey[300] : e.palette.grey[700], r = Ye(e.palette.text.primary, 0.26);
  return {
    /* Styles applied to the root element. */
    root: {
      fontFamily: e.typography.fontFamily,
      fontSize: e.typography.pxToRem(13),
      display: "inline-flex",
      alignItems: "center",
      justifyContent: "center",
      height: 32,
      color: e.palette.getContrastText(a),
      backgroundColor: a,
      borderRadius: 32 / 2,
      whiteSpace: "nowrap",
      transition: e.transitions.create(["background-color", "box-shadow"]),
      // label will inherit this from root, then `clickable` class overrides this for both
      cursor: "default",
      // We disable the focus ring for mouse, touch and keyboard users.
      outline: 0,
      textDecoration: "none",
      border: "none",
      // Remove `button` border
      padding: 0,
      // Remove `button` padding
      verticalAlign: "middle",
      boxSizing: "border-box",
      "&$disabled": {
        opacity: 0.5,
        pointerEvents: "none"
      },
      "& $avatar": {
        marginLeft: 5,
        marginRight: -6,
        width: 24,
        height: 24,
        color: e.palette.type === "light" ? e.palette.grey[700] : e.palette.grey[300],
        fontSize: e.typography.pxToRem(12)
      },
      "& $avatarColorPrimary": {
        color: e.palette.primary.contrastText,
        backgroundColor: e.palette.primary.dark
      },
      "& $avatarColorSecondary": {
        color: e.palette.secondary.contrastText,
        backgroundColor: e.palette.secondary.dark
      },
      "& $avatarSmall": {
        marginLeft: 4,
        marginRight: -4,
        width: 18,
        height: 18,
        fontSize: e.typography.pxToRem(10)
      }
    },
    /* Styles applied to the root element if `size="small"`. */
    sizeSmall: {
      height: 24
    },
    /* Styles applied to the root element if `color="primary"`. */
    colorPrimary: {
      backgroundColor: e.palette.primary.main,
      color: e.palette.primary.contrastText
    },
    /* Styles applied to the root element if `color="secondary"`. */
    colorSecondary: {
      backgroundColor: e.palette.secondary.main,
      color: e.palette.secondary.contrastText
    },
    /* Pseudo-class applied to the root element if `disabled={true}`. */
    disabled: {},
    /* Styles applied to the root element if `onClick` is defined or `clickable={true}`. */
    clickable: {
      userSelect: "none",
      WebkitTapHighlightColor: "transparent",
      cursor: "pointer",
      "&:hover, &:focus": {
        backgroundColor: rn(a, 0.08)
      },
      "&:active": {
        boxShadow: e.shadows[1]
      }
    },
    /* Styles applied to the root element if `onClick` and `color="primary"` is defined or `clickable={true}`. */
    clickableColorPrimary: {
      "&:hover, &:focus": {
        backgroundColor: rn(e.palette.primary.main, 0.08)
      }
    },
    /* Styles applied to the root element if `onClick` and `color="secondary"` is defined or `clickable={true}`. */
    clickableColorSecondary: {
      "&:hover, &:focus": {
        backgroundColor: rn(e.palette.secondary.main, 0.08)
      }
    },
    /* Styles applied to the root element if `onDelete` is defined. */
    deletable: {
      "&:focus": {
        backgroundColor: rn(a, 0.08)
      }
    },
    /* Styles applied to the root element if `onDelete` and `color="primary"` is defined. */
    deletableColorPrimary: {
      "&:focus": {
        backgroundColor: rn(e.palette.primary.main, 0.2)
      }
    },
    /* Styles applied to the root element if `onDelete` and `color="secondary"` is defined. */
    deletableColorSecondary: {
      "&:focus": {
        backgroundColor: rn(e.palette.secondary.main, 0.2)
      }
    },
    /* Styles applied to the root element if `variant="outlined"`. */
    outlined: {
      backgroundColor: "transparent",
      border: "1px solid ".concat(e.palette.type === "light" ? "rgba(0, 0, 0, 0.23)" : "rgba(255, 255, 255, 0.23)"),
      "$clickable&:hover, $clickable&:focus, $deletable&:focus": {
        backgroundColor: Ye(e.palette.text.primary, e.palette.action.hoverOpacity)
      },
      "& $avatar": {
        marginLeft: 4
      },
      "& $avatarSmall": {
        marginLeft: 2
      },
      "& $icon": {
        marginLeft: 4
      },
      "& $iconSmall": {
        marginLeft: 2
      },
      "& $deleteIcon": {
        marginRight: 5
      },
      "& $deleteIconSmall": {
        marginRight: 3
      }
    },
    /* Styles applied to the root element if `variant="outlined"` and `color="primary"`. */
    outlinedPrimary: {
      color: e.palette.primary.main,
      border: "1px solid ".concat(e.palette.primary.main),
      "$clickable&:hover, $clickable&:focus, $deletable&:focus": {
        backgroundColor: Ye(e.palette.primary.main, e.palette.action.hoverOpacity)
      }
    },
    /* Styles applied to the root element if `variant="outlined"` and `color="secondary"`. */
    outlinedSecondary: {
      color: e.palette.secondary.main,
      border: "1px solid ".concat(e.palette.secondary.main),
      "$clickable&:hover, $clickable&:focus, $deletable&:focus": {
        backgroundColor: Ye(e.palette.secondary.main, e.palette.action.hoverOpacity)
      }
    },
    // TODO v5: remove
    /* Styles applied to the `avatar` element. */
    avatar: {},
    /* Styles applied to the `avatar` element if `size="small"`. */
    avatarSmall: {},
    /* Styles applied to the `avatar` element if `color="primary"`. */
    avatarColorPrimary: {},
    /* Styles applied to the `avatar` element if `color="secondary"`. */
    avatarColorSecondary: {},
    /* Styles applied to the `icon` element. */
    icon: {
      color: e.palette.type === "light" ? e.palette.grey[700] : e.palette.grey[300],
      marginLeft: 5,
      marginRight: -6
    },
    /* Styles applied to the `icon` element if `size="small"`. */
    iconSmall: {
      width: 18,
      height: 18,
      marginLeft: 4,
      marginRight: -4
    },
    /* Styles applied to the `icon` element if `color="primary"`. */
    iconColorPrimary: {
      color: "inherit"
    },
    /* Styles applied to the `icon` element if `color="secondary"`. */
    iconColorSecondary: {
      color: "inherit"
    },
    /* Styles applied to the label `span` element. */
    label: {
      overflow: "hidden",
      textOverflow: "ellipsis",
      paddingLeft: 12,
      paddingRight: 12,
      whiteSpace: "nowrap"
    },
    /* Styles applied to the label `span` element if `size="small"`. */
    labelSmall: {
      paddingLeft: 8,
      paddingRight: 8
    },
    /* Styles applied to the `deleteIcon` element. */
    deleteIcon: {
      WebkitTapHighlightColor: "transparent",
      color: r,
      height: 22,
      width: 22,
      cursor: "pointer",
      margin: "0 5px 0 -6px",
      "&:hover": {
        color: Ye(r, 0.4)
      }
    },
    /* Styles applied to the `deleteIcon` element if `size="small"`. */
    deleteIconSmall: {
      height: 16,
      width: 16,
      marginRight: 4,
      marginLeft: -4
    },
    /* Styles applied to the deleteIcon element if `color="primary"` and `variant="default"`. */
    deleteIconColorPrimary: {
      color: Ye(e.palette.primary.contrastText, 0.7),
      "&:hover, &:active": {
        color: e.palette.primary.contrastText
      }
    },
    /* Styles applied to the deleteIcon element if `color="secondary"` and `variant="default"`. */
    deleteIconColorSecondary: {
      color: Ye(e.palette.secondary.contrastText, 0.7),
      "&:hover, &:active": {
        color: e.palette.secondary.contrastText
      }
    },
    /* Styles applied to the deleteIcon element if `color="primary"` and `variant="outlined"`. */
    deleteIconOutlinedColorPrimary: {
      color: Ye(e.palette.primary.main, 0.7),
      "&:hover, &:active": {
        color: e.palette.primary.main
      }
    },
    /* Styles applied to the deleteIcon element if `color="secondary"` and `variant="outlined"`. */
    deleteIconOutlinedColorSecondary: {
      color: Ye(e.palette.secondary.main, 0.7),
      "&:hover, &:active": {
        color: e.palette.secondary.main
      }
    }
  };
};
function bl(t) {
  return t.key === "Backspace" || t.key === "Delete";
}
var Ag = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.avatar, n = e.classes, o = e.className, i = e.clickable, s = e.color, l = s === void 0 ? "default" : s, u = e.component, c = e.deleteIcon, d = e.disabled, p = d === void 0 ? !1 : d, h = e.icon, g = e.label, m = e.onClick, y = e.onDelete, w = e.onKeyDown, k = e.onKeyUp, j = e.size, f = j === void 0 ? "medium" : j, x = e.variant, E = x === void 0 ? "default" : x, L = Ae(e, ["avatar", "classes", "className", "clickable", "color", "component", "deleteIcon", "disabled", "icon", "label", "onClick", "onDelete", "onKeyDown", "onKeyUp", "size", "variant"]), W = b.useRef(null), O = St(W, a), P = function(Q) {
    Q.stopPropagation(), y && y(Q);
  }, A = function(Q) {
    Q.currentTarget === Q.target && bl(Q) && Q.preventDefault(), w && w(Q);
  }, v = function(Q) {
    Q.currentTarget === Q.target && (y && bl(Q) ? y(Q) : Q.key === "Escape" && W.current && W.current.blur()), k && k(Q);
  }, C = i !== !1 && m ? !0 : i, M = f === "small", D = u || (C ? kr : "div"), $ = D === kr ? {
    component: "div"
  } : {}, H = null;
  if (y) {
    var F = he(l !== "default" && (E === "default" ? n["deleteIconColor".concat(pt(l))] : n["deleteIconOutlinedColor".concat(pt(l))]), M && n.deleteIconSmall);
    H = c && /* @__PURE__ */ b.isValidElement(c) ? /* @__PURE__ */ b.cloneElement(c, {
      className: he(c.props.className, n.deleteIcon, F),
      onClick: P
    }) : /* @__PURE__ */ b.createElement(jg, {
      className: he(n.deleteIcon, F),
      onClick: P
    });
  }
  var q = null;
  r && /* @__PURE__ */ b.isValidElement(r) && (q = /* @__PURE__ */ b.cloneElement(r, {
    className: he(n.avatar, r.props.className, M && n.avatarSmall, l !== "default" && n["avatarColor".concat(pt(l))])
  }));
  var z = null;
  return h && /* @__PURE__ */ b.isValidElement(h) && (z = /* @__PURE__ */ b.cloneElement(h, {
    className: he(n.icon, h.props.className, M && n.iconSmall, l !== "default" && n["iconColor".concat(pt(l))])
  })), /* @__PURE__ */ b.createElement(D, Y({
    role: C || y ? "button" : void 0,
    className: he(n.root, o, l !== "default" && [n["color".concat(pt(l))], C && n["clickableColor".concat(pt(l))], y && n["deletableColor".concat(pt(l))]], E !== "default" && [n.outlined, {
      primary: n.outlinedPrimary,
      secondary: n.outlinedSecondary
    }[l]], p && n.disabled, M && n.sizeSmall, C && n.clickable, y && n.deletable),
    "aria-disabled": p ? !0 : void 0,
    tabIndex: C || y ? 0 : void 0,
    onClick: m,
    onKeyDown: A,
    onKeyUp: v,
    ref: O
  }, $, L), q || z, /* @__PURE__ */ b.createElement("span", {
    className: he(n.label, M && n.labelSmall)
  }, g), H);
});
const Mg = et(Ig, {
  name: "MuiChip"
})(Ag);
var vr = 44, Ng = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      display: "inline-block"
    },
    /* Styles applied to the root element if `variant="static"`. */
    static: {
      transition: e.transitions.create("transform")
    },
    /* Styles applied to the root element if `variant="indeterminate"`. */
    indeterminate: {
      animation: "$circular-rotate 1.4s linear infinite"
    },
    /* Styles applied to the root element if `variant="determinate"`. */
    determinate: {
      transition: e.transitions.create("transform")
    },
    /* Styles applied to the root element if `color="primary"`. */
    colorPrimary: {
      color: e.palette.primary.main
    },
    /* Styles applied to the root element if `color="secondary"`. */
    colorSecondary: {
      color: e.palette.secondary.main
    },
    /* Styles applied to the `svg` element. */
    svg: {
      display: "block"
      // Keeps the progress centered
    },
    /* Styles applied to the `circle` svg path. */
    circle: {
      stroke: "currentColor"
      // Use butt to follow the specification, by chance, it's already the default CSS value.
      // strokeLinecap: 'butt',
    },
    /* Styles applied to the `circle` svg path if `variant="static"`. */
    circleStatic: {
      transition: e.transitions.create("stroke-dashoffset")
    },
    /* Styles applied to the `circle` svg path if `variant="indeterminate"`. */
    circleIndeterminate: {
      animation: "$circular-dash 1.4s ease-in-out infinite",
      // Some default value that looks fine waiting for the animation to kicks in.
      strokeDasharray: "80px, 200px",
      strokeDashoffset: "0px"
      // Add the unit to fix a Edge 16 and below bug.
    },
    /* Styles applied to the `circle` svg path if `variant="determinate"`. */
    circleDeterminate: {
      transition: e.transitions.create("stroke-dashoffset")
    },
    "@keyframes circular-rotate": {
      "0%": {
        // Fix IE 11 wobbly
        transformOrigin: "50% 50%"
      },
      "100%": {
        transform: "rotate(360deg)"
      }
    },
    "@keyframes circular-dash": {
      "0%": {
        strokeDasharray: "1px, 200px",
        strokeDashoffset: "0px"
      },
      "50%": {
        strokeDasharray: "100px, 200px",
        strokeDashoffset: "-15px"
      },
      "100%": {
        strokeDasharray: "100px, 200px",
        strokeDashoffset: "-125px"
      }
    },
    /* Styles applied to the `circle` svg path if `disableShrink={true}`. */
    circleDisableShrink: {
      animation: "none"
    }
  };
}, Lg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.classes, n = e.className, o = e.color, i = o === void 0 ? "primary" : o, s = e.disableShrink, l = s === void 0 ? !1 : s, u = e.size, c = u === void 0 ? 40 : u, d = e.style, p = e.thickness, h = p === void 0 ? 3.6 : p, g = e.value, m = g === void 0 ? 0 : g, y = e.variant, w = y === void 0 ? "indeterminate" : y, k = Ae(e, ["classes", "className", "color", "disableShrink", "size", "style", "thickness", "value", "variant"]), j = {}, f = {}, x = {};
  if (w === "determinate" || w === "static") {
    var E = 2 * Math.PI * ((vr - h) / 2);
    j.strokeDasharray = E.toFixed(3), x["aria-valuenow"] = Math.round(m), j.strokeDashoffset = "".concat(((100 - m) / 100 * E).toFixed(3), "px"), f.transform = "rotate(-90deg)";
  }
  return /* @__PURE__ */ b.createElement("div", Y({
    className: he(r.root, n, i !== "inherit" && r["color".concat(pt(i))], {
      determinate: r.determinate,
      indeterminate: r.indeterminate,
      static: r.static
    }[w]),
    style: Y({
      width: c,
      height: c
    }, f, d),
    ref: a,
    role: "progressbar"
  }, x, k), /* @__PURE__ */ b.createElement("svg", {
    className: r.svg,
    viewBox: "".concat(vr / 2, " ").concat(vr / 2, " ").concat(vr, " ").concat(vr)
  }, /* @__PURE__ */ b.createElement("circle", {
    className: he(r.circle, l && r.circleDisableShrink, {
      determinate: r.circleDeterminate,
      indeterminate: r.circleIndeterminate,
      static: r.circleStatic
    }[w]),
    style: j,
    cx: vr,
    cy: vr,
    r: (vr - h) / 2,
    fill: "none",
    strokeWidth: h
  })));
});
const Xn = et(Ng, {
  name: "MuiCircularProgress",
  flip: !1
})(Lg);
function Dg(t) {
  return t = typeof t == "function" ? t() : t, Ut.findDOMNode(t);
}
var Jo = typeof window < "u" ? b.useLayoutEffect : b.useEffect, cu = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.container, o = e.disablePortal, i = o === void 0 ? !1 : o, s = e.onRendered, l = b.useState(null), u = l[0], c = l[1], d = St(/* @__PURE__ */ b.isValidElement(r) ? r.ref : null, a);
  return Jo(function() {
    i || c(Dg(n) || document.body);
  }, [n, i]), Jo(function() {
    if (u && !i)
      return Hr(a, u), function() {
        Hr(a, null);
      };
  }, [a, u, i]), Jo(function() {
    s && (u || i) && s();
  }, [s, u, i]), i ? /* @__PURE__ */ b.isValidElement(r) ? /* @__PURE__ */ b.cloneElement(r, {
    ref: d
  }) : r : u && /* @__PURE__ */ Ut.createPortal(r, u);
});
function uu() {
  var t = document.createElement("div");
  t.style.width = "99px", t.style.height = "99px", t.style.position = "absolute", t.style.top = "-9999px", t.style.overflow = "scroll", document.body.appendChild(t);
  var e = t.offsetWidth - t.clientWidth;
  return document.body.removeChild(t), e;
}
function Bg(t) {
  var e = cr(t);
  return e.body === t ? rs(e).innerWidth > e.documentElement.clientWidth : t.scrollHeight > t.clientHeight;
}
function Kn(t, e) {
  e ? t.setAttribute("aria-hidden", "true") : t.removeAttribute("aria-hidden");
}
function yl(t) {
  return parseInt(window.getComputedStyle(t)["padding-right"], 10) || 0;
}
function xl(t, e, a) {
  var r = arguments.length > 3 && arguments[3] !== void 0 ? arguments[3] : [], n = arguments.length > 4 ? arguments[4] : void 0, o = [e, a].concat(Xa(r)), i = ["TEMPLATE", "SCRIPT", "STYLE"];
  [].forEach.call(t.children, function(s) {
    s.nodeType === 1 && o.indexOf(s) === -1 && i.indexOf(s.tagName) === -1 && Kn(s, n);
  });
}
function Zo(t, e) {
  var a = -1;
  return t.some(function(r, n) {
    return e(r) ? (a = n, !0) : !1;
  }), a;
}
function Fg(t, e) {
  var a = [], r = [], n = t.container, o;
  if (!e.disableScrollLock) {
    if (Bg(n)) {
      var i = uu();
      a.push({
        value: n.style.paddingRight,
        key: "padding-right",
        el: n
      }), n.style["padding-right"] = "".concat(yl(n) + i, "px"), o = cr(n).querySelectorAll(".mui-fixed"), [].forEach.call(o, function(c) {
        r.push(c.style.paddingRight), c.style.paddingRight = "".concat(yl(c) + i, "px");
      });
    }
    var s = n.parentElement, l = s.nodeName === "HTML" && window.getComputedStyle(s)["overflow-y"] === "scroll" ? s : n;
    a.push({
      value: l.style.overflow,
      key: "overflow",
      el: l
    }), l.style.overflow = "hidden";
  }
  var u = function() {
    o && [].forEach.call(o, function(d, p) {
      r[p] ? d.style.paddingRight = r[p] : d.style.removeProperty("padding-right");
    }), a.forEach(function(d) {
      var p = d.value, h = d.el, g = d.key;
      p ? h.style.setProperty(g, p) : h.style.removeProperty(g);
    });
  };
  return u;
}
function zg(t) {
  var e = [];
  return [].forEach.call(t.children, function(a) {
    a.getAttribute && a.getAttribute("aria-hidden") === "true" && e.push(a);
  }), e;
}
var Wg = /* @__PURE__ */ (function() {
  function t() {
    Pm(this, t), this.modals = [], this.containers = [];
  }
  return Gi(t, [{
    key: "add",
    value: function(a, r) {
      var n = this.modals.indexOf(a);
      if (n !== -1)
        return n;
      n = this.modals.length, this.modals.push(a), a.modalRef && Kn(a.modalRef, !1);
      var o = zg(r);
      xl(r, a.mountNode, a.modalRef, o, !0);
      var i = Zo(this.containers, function(s) {
        return s.container === r;
      });
      return i !== -1 ? (this.containers[i].modals.push(a), n) : (this.containers.push({
        modals: [a],
        container: r,
        restore: null,
        hiddenSiblingNodes: o
      }), n);
    }
  }, {
    key: "mount",
    value: function(a, r) {
      var n = Zo(this.containers, function(i) {
        return i.modals.indexOf(a) !== -1;
      }), o = this.containers[n];
      o.restore || (o.restore = Fg(o, r));
    }
  }, {
    key: "remove",
    value: function(a) {
      var r = this.modals.indexOf(a);
      if (r === -1)
        return r;
      var n = Zo(this.containers, function(s) {
        return s.modals.indexOf(a) !== -1;
      }), o = this.containers[n];
      if (o.modals.splice(o.modals.indexOf(a), 1), this.modals.splice(r, 1), o.modals.length === 0)
        o.restore && o.restore(), a.modalRef && Kn(a.modalRef, !0), xl(o.container, a.mountNode, a.modalRef, o.hiddenSiblingNodes, !1), this.containers.splice(n, 1);
      else {
        var i = o.modals[o.modals.length - 1];
        i.modalRef && Kn(i.modalRef, !1);
      }
      return r;
    }
  }, {
    key: "isTopModal",
    value: function(a) {
      return this.modals.length > 0 && this.modals[this.modals.length - 1] === a;
    }
  }]), t;
})();
function Vg(t) {
  var e = t.children, a = t.disableAutoFocus, r = a === void 0 ? !1 : a, n = t.disableEnforceFocus, o = n === void 0 ? !1 : n, i = t.disableRestoreFocus, s = i === void 0 ? !1 : i, l = t.getDoc, u = t.isEnabled, c = t.open, d = b.useRef(), p = b.useRef(null), h = b.useRef(null), g = b.useRef(), m = b.useRef(null), y = b.useCallback(function(j) {
    m.current = Ut.findDOMNode(j);
  }, []), w = St(e.ref, y), k = b.useRef();
  return b.useEffect(function() {
    k.current = c;
  }, [c]), !k.current && c && typeof window < "u" && (g.current = l().activeElement), b.useEffect(function() {
    if (c) {
      var j = cr(m.current);
      !r && m.current && !m.current.contains(j.activeElement) && (m.current.hasAttribute("tabIndex") || m.current.setAttribute("tabIndex", -1), m.current.focus());
      var f = function() {
        var W = m.current;
        if (W !== null) {
          if (!j.hasFocus() || o || !u() || d.current) {
            d.current = !1;
            return;
          }
          m.current && !m.current.contains(j.activeElement) && m.current.focus();
        }
      }, x = function(W) {
        o || !u() || W.keyCode !== 9 || j.activeElement === m.current && (d.current = !0, W.shiftKey ? h.current.focus() : p.current.focus());
      };
      j.addEventListener("focus", f, !0), j.addEventListener("keydown", x, !0);
      var E = setInterval(function() {
        f();
      }, 50);
      return function() {
        clearInterval(E), j.removeEventListener("focus", f, !0), j.removeEventListener("keydown", x, !0), s || (g.current && g.current.focus && g.current.focus(), g.current = null);
      };
    }
  }, [r, o, s, u, c]), /* @__PURE__ */ b.createElement(b.Fragment, null, /* @__PURE__ */ b.createElement("div", {
    tabIndex: 0,
    ref: p,
    "data-test": "sentinelStart"
  }), /* @__PURE__ */ b.cloneElement(e, {
    ref: w
  }), /* @__PURE__ */ b.createElement("div", {
    tabIndex: 0,
    ref: h,
    "data-test": "sentinelEnd"
  }));
}
var Cl = {
  /* Styles applied to the root element. */
  root: {
    zIndex: -1,
    position: "fixed",
    right: 0,
    bottom: 0,
    top: 0,
    left: 0,
    backgroundColor: "rgba(0, 0, 0, 0.5)",
    WebkitTapHighlightColor: "transparent"
  },
  /* Styles applied to the root element if `invisible={true}`. */
  invisible: {
    backgroundColor: "transparent"
  }
}, Hg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.invisible, n = r === void 0 ? !1 : r, o = e.open, i = Ae(e, ["invisible", "open"]);
  return o ? /* @__PURE__ */ b.createElement("div", Y({
    "aria-hidden": !0,
    ref: a
  }, i, {
    style: Y({}, Cl.root, n ? Cl.invisible : {}, i.style)
  })) : null;
});
function Kg(t) {
  return t = typeof t == "function" ? t() : t, Ut.findDOMNode(t);
}
function Ug(t) {
  return t.children ? t.children.props.hasOwnProperty("in") : !1;
}
var qg = new Wg(), Gg = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      position: "fixed",
      zIndex: e.zIndex.modal,
      right: 0,
      bottom: 0,
      top: 0,
      left: 0
    },
    /* Styles applied to the root element if the `Modal` has exited. */
    hidden: {
      visibility: "hidden"
    }
  };
}, Yg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = na(), n = Fc({
    name: "MuiModal",
    props: Y({}, e),
    theme: r
  }), o = n.BackdropComponent, i = o === void 0 ? Hg : o, s = n.BackdropProps, l = n.children, u = n.closeAfterTransition, c = u === void 0 ? !1 : u, d = n.container, p = n.disableAutoFocus, h = p === void 0 ? !1 : p, g = n.disableBackdropClick, m = g === void 0 ? !1 : g, y = n.disableEnforceFocus, w = y === void 0 ? !1 : y, k = n.disableEscapeKeyDown, j = k === void 0 ? !1 : k, f = n.disablePortal, x = f === void 0 ? !1 : f, E = n.disableRestoreFocus, L = E === void 0 ? !1 : E, W = n.disableScrollLock, O = W === void 0 ? !1 : W, P = n.hideBackdrop, A = P === void 0 ? !1 : P, v = n.keepMounted, C = v === void 0 ? !1 : v, M = n.manager, D = M === void 0 ? qg : M, $ = n.onBackdropClick, H = n.onClose, F = n.onEscapeKeyDown, q = n.onRendered, z = n.open, Z = Ae(n, ["BackdropComponent", "BackdropProps", "children", "closeAfterTransition", "container", "disableAutoFocus", "disableBackdropClick", "disableEnforceFocus", "disableEscapeKeyDown", "disablePortal", "disableRestoreFocus", "disableScrollLock", "hideBackdrop", "keepMounted", "manager", "onBackdropClick", "onClose", "onEscapeKeyDown", "onRendered", "open"]), Q = b.useState(!0), ue = Q[0], de = Q[1], Se = b.useRef({}), fe = b.useRef(null), ye = b.useRef(null), be = St(ye, a), we = Ug(n), Ee = function() {
    return cr(fe.current);
  }, ge = function() {
    return Se.current.modalRef = ye.current, Se.current.mountNode = fe.current, Se.current;
  }, ke = function() {
    D.mount(ge(), {
      disableScrollLock: O
    }), ye.current.scrollTop = 0;
  }, je = rr(function() {
    var ce = Kg(d) || Ee().body;
    D.add(ge(), ce), ye.current && ke();
  }), Te = b.useCallback(function() {
    return D.isTopModal(ge());
  }, [D]), He = rr(function(ce) {
    fe.current = ce, ce && (q && q(), z && Te() ? ke() : Kn(ye.current, !0));
  }), _e = b.useCallback(function() {
    D.remove(ge());
  }, [D]);
  if (b.useEffect(function() {
    return function() {
      _e();
    };
  }, [_e]), b.useEffect(function() {
    z ? je() : (!we || !c) && _e();
  }, [z, _e, we, c, je]), !C && !z && (!we || ue))
    return null;
  var Fe = function() {
    de(!1);
  }, We = function() {
    de(!0), c && _e();
  }, Ue = function(pe) {
    pe.target === pe.currentTarget && ($ && $(pe), !m && H && H(pe, "backdropClick"));
  }, tt = function(pe) {
    pe.key !== "Escape" || !Te() || (F && F(pe), j || (pe.stopPropagation(), H && H(pe, "escapeKeyDown")));
  }, Qe = Gg(r || {
    zIndex: Bc
  }), Ve = {};
  return l.props.tabIndex === void 0 && (Ve.tabIndex = l.props.tabIndex || "-1"), we && (Ve.onEnter = Gn(Fe, l.props.onEnter), Ve.onExited = Gn(We, l.props.onExited)), /* @__PURE__ */ b.createElement(cu, {
    ref: He,
    container: d,
    disablePortal: x
  }, /* @__PURE__ */ b.createElement("div", Y({
    ref: be,
    onKeyDown: tt,
    role: "presentation"
  }, Z, {
    style: Y({}, Qe.root, !z && ue ? Qe.hidden : {}, Z.style)
  }), A ? null : /* @__PURE__ */ b.createElement(i, Y({
    open: z,
    onClick: Ue
  }, s)), /* @__PURE__ */ b.createElement(Vg, {
    disableEnforceFocus: w,
    disableAutoFocus: h,
    disableRestoreFocus: L,
    getDoc: Ee,
    isEnabled: Te,
    open: z
  }, /* @__PURE__ */ b.cloneElement(l, Ve))));
}), Xg = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      height: 1,
      margin: 0,
      // Reset browser default style.
      border: "none",
      flexShrink: 0,
      backgroundColor: e.palette.divider
    },
    /* Styles applied to the root element if `absolute={true}`. */
    absolute: {
      position: "absolute",
      bottom: 0,
      left: 0,
      width: "100%"
    },
    /* Styles applied to the root element if `variant="inset"`. */
    inset: {
      marginLeft: 72
    },
    /* Styles applied to the root element if `light={true}`. */
    light: {
      backgroundColor: Ye(e.palette.divider, 0.08)
    },
    /* Styles applied to the root element if `variant="middle"`. */
    middle: {
      marginLeft: e.spacing(2),
      marginRight: e.spacing(2)
    },
    /* Styles applied to the root element if `orientation="vertical"`. */
    vertical: {
      height: "100%",
      width: 1
    },
    /* Styles applied to the root element if `flexItem={true}`. */
    flexItem: {
      alignSelf: "stretch",
      height: "auto"
    }
  };
}, Jg = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.absolute, n = r === void 0 ? !1 : r, o = e.classes, i = e.className, s = e.component, l = s === void 0 ? "hr" : s, u = e.flexItem, c = u === void 0 ? !1 : u, d = e.light, p = d === void 0 ? !1 : d, h = e.orientation, g = h === void 0 ? "horizontal" : h, m = e.role, y = m === void 0 ? l !== "hr" ? "separator" : void 0 : m, w = e.variant, k = w === void 0 ? "fullWidth" : w, j = Ae(e, ["absolute", "classes", "className", "component", "flexItem", "light", "orientation", "role", "variant"]);
  return /* @__PURE__ */ b.createElement(l, Y({
    className: he(o.root, i, k !== "fullWidth" && o[k], n && o.absolute, c && o.flexItem, p && o.light, g === "vertical" && o.vertical),
    role: y,
    ref: a
  }, j));
});
const Zg = et(Xg, {
  name: "MuiDivider"
})(Jg);
function to(t) {
  var e = t.props, a = t.states, r = t.muiFormControl;
  return a.reduce(function(n, o) {
    return n[o] = e[o], r && typeof e[o] > "u" && (n[o] = r[o]), n;
  }, {});
}
function ya(t, e) {
  return parseInt(t[e], 10) || 0;
}
var Qg = typeof window < "u" ? b.useLayoutEffect : b.useEffect, e0 = {
  /* Styles applied to the shadow textarea element. */
  shadow: {
    // Visibility needed to hide the extra text area on iPads
    visibility: "hidden",
    // Remove from the content flow
    position: "absolute",
    // Ignore the scrollbar width
    overflow: "hidden",
    height: 0,
    top: 0,
    left: 0,
    // Create a new layer, increase the isolation of the computed values
    transform: "translateZ(0)"
  }
}, t0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.onChange, n = e.rows, o = e.rowsMax, i = e.rowsMin, s = e.maxRows, l = e.minRows, u = l === void 0 ? 1 : l, c = e.style, d = e.value, p = Ae(e, ["onChange", "rows", "rowsMax", "rowsMin", "maxRows", "minRows", "style", "value"]), h = s || o, g = n || i || u, m = b.useRef(d != null), y = m.current, w = b.useRef(null), k = St(a, w), j = b.useRef(null), f = b.useRef(0), x = b.useState({}), E = x[0], L = x[1], W = b.useCallback(function() {
    var P = w.current, A = window.getComputedStyle(P), v = j.current;
    v.style.width = A.width, v.value = P.value || e.placeholder || "x", v.value.slice(-1) === `
` && (v.value += " ");
    var C = A["box-sizing"], M = ya(A, "padding-bottom") + ya(A, "padding-top"), D = ya(A, "border-bottom-width") + ya(A, "border-top-width"), $ = v.scrollHeight - M;
    v.value = "x";
    var H = v.scrollHeight - M, F = $;
    g && (F = Math.max(Number(g) * H, F)), h && (F = Math.min(Number(h) * H, F)), F = Math.max(F, H);
    var q = F + (C === "border-box" ? M + D : 0), z = Math.abs(F - $) <= 1;
    L(function(Z) {
      return f.current < 20 && (q > 0 && Math.abs((Z.outerHeightStyle || 0) - q) > 1 || Z.overflow !== z) ? (f.current += 1, {
        overflow: z,
        outerHeightStyle: q
      }) : Z;
    });
  }, [h, g, e.placeholder]);
  b.useEffect(function() {
    var P = Yn(function() {
      f.current = 0, W();
    });
    return window.addEventListener("resize", P), function() {
      P.clear(), window.removeEventListener("resize", P);
    };
  }, [W]), Qg(function() {
    W();
  }), b.useEffect(function() {
    f.current = 0;
  }, [d]);
  var O = function(A) {
    f.current = 0, y || W(), r && r(A);
  };
  return /* @__PURE__ */ b.createElement(b.Fragment, null, /* @__PURE__ */ b.createElement("textarea", Y({
    value: d,
    onChange: O,
    ref: k,
    rows: g,
    style: Y({
      height: E.outerHeightStyle,
      // Need a large enough difference to allow scrolling.
      // This prevents infinite rendering loop.
      overflow: E.overflow ? "hidden" : null
    }, c)
  }, p)), /* @__PURE__ */ b.createElement("textarea", {
    "aria-hidden": !0,
    className: e.className,
    readOnly: !0,
    ref: j,
    tabIndex: -1,
    style: Y({}, e0.shadow, c)
  }));
});
function Sl(t) {
  return t != null && !(Array.isArray(t) && t.length === 0);
}
function ls(t) {
  var e = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : !1;
  return t && (Sl(t.value) && t.value !== "" || e && Sl(t.defaultValue) && t.defaultValue !== "");
}
function r0(t) {
  return t.startAdornment;
}
var n0 = function(e) {
  var a = e.palette.type === "light", r = {
    color: "currentColor",
    opacity: a ? 0.42 : 0.5,
    transition: e.transitions.create("opacity", {
      duration: e.transitions.duration.shorter
    })
  }, n = {
    opacity: "0 !important"
  }, o = {
    opacity: a ? 0.42 : 0.5
  };
  return {
    "@global": {
      "@keyframes mui-auto-fill": {},
      "@keyframes mui-auto-fill-cancel": {}
    },
    /* Styles applied to the root element. */
    root: Y({}, e.typography.body1, {
      color: e.palette.text.primary,
      lineHeight: "1.1876em",
      // Reset (19px), match the native input line-height
      boxSizing: "border-box",
      // Prevent padding issue with fullWidth.
      position: "relative",
      cursor: "text",
      display: "inline-flex",
      alignItems: "center",
      "&$disabled": {
        color: e.palette.text.disabled,
        cursor: "default"
      }
    }),
    /* Styles applied to the root element if the component is a descendant of `FormControl`. */
    formControl: {},
    /* Styles applied to the root element if the component is focused. */
    focused: {},
    /* Styles applied to the root element if `disabled={true}`. */
    disabled: {},
    /* Styles applied to the root element if `startAdornment` is provided. */
    adornedStart: {},
    /* Styles applied to the root element if `endAdornment` is provided. */
    adornedEnd: {},
    /* Pseudo-class applied to the root element if `error={true}`. */
    error: {},
    /* Styles applied to the `input` element if `margin="dense"`. */
    marginDense: {},
    /* Styles applied to the root element if `multiline={true}`. */
    multiline: {
      padding: "".concat(6, "px 0 ").concat(7, "px"),
      "&$marginDense": {
        paddingTop: 3
      }
    },
    /* Styles applied to the root element if the color is secondary. */
    colorSecondary: {},
    /* Styles applied to the root element if `fullWidth={true}`. */
    fullWidth: {
      width: "100%"
    },
    /* Styles applied to the `input` element. */
    input: {
      font: "inherit",
      letterSpacing: "inherit",
      color: "currentColor",
      padding: "".concat(6, "px 0 ").concat(7, "px"),
      border: 0,
      boxSizing: "content-box",
      background: "none",
      height: "1.1876em",
      // Reset (19px), match the native input line-height
      margin: 0,
      // Reset for Safari
      WebkitTapHighlightColor: "transparent",
      display: "block",
      // Make the flex item shrink with Firefox
      minWidth: 0,
      width: "100%",
      // Fix IE 11 width issue
      animationName: "mui-auto-fill-cancel",
      animationDuration: "10ms",
      "&::-webkit-input-placeholder": r,
      "&::-moz-placeholder": r,
      // Firefox 19+
      "&:-ms-input-placeholder": r,
      // IE 11
      "&::-ms-input-placeholder": r,
      // Edge
      "&:focus": {
        outline: 0
      },
      // Reset Firefox invalid required input style
      "&:invalid": {
        boxShadow: "none"
      },
      "&::-webkit-search-decoration": {
        // Remove the padding when type=search.
        "-webkit-appearance": "none"
      },
      // Show and hide the placeholder logic
      "label[data-shrink=false] + $formControl &": {
        "&::-webkit-input-placeholder": n,
        "&::-moz-placeholder": n,
        // Firefox 19+
        "&:-ms-input-placeholder": n,
        // IE 11
        "&::-ms-input-placeholder": n,
        // Edge
        "&:focus::-webkit-input-placeholder": o,
        "&:focus::-moz-placeholder": o,
        // Firefox 19+
        "&:focus:-ms-input-placeholder": o,
        // IE 11
        "&:focus::-ms-input-placeholder": o
        // Edge
      },
      "&$disabled": {
        opacity: 1
        // Reset iOS opacity
      },
      "&:-webkit-autofill": {
        animationDuration: "5000s",
        animationName: "mui-auto-fill"
      }
    },
    /* Styles applied to the `input` element if `margin="dense"`. */
    inputMarginDense: {
      paddingTop: 3
    },
    /* Styles applied to the `input` element if `multiline={true}`. */
    inputMultiline: {
      height: "auto",
      resize: "none",
      padding: 0
    },
    /* Styles applied to the `input` element if `type="search"`. */
    inputTypeSearch: {
      // Improve type search style.
      "-moz-appearance": "textfield",
      "-webkit-appearance": "textfield"
    },
    /* Styles applied to the `input` element if `startAdornment` is provided. */
    inputAdornedStart: {},
    /* Styles applied to the `input` element if `endAdornment` is provided. */
    inputAdornedEnd: {},
    /* Styles applied to the `input` element if `hiddenLabel={true}`. */
    inputHiddenLabel: {}
  };
}, a0 = typeof window > "u" ? b.useEffect : b.useLayoutEffect, o0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e["aria-describedby"], n = e.autoComplete, o = e.autoFocus, i = e.classes, s = e.className;
  e.color;
  var l = e.defaultValue, u = e.disabled, c = e.endAdornment;
  e.error;
  var d = e.fullWidth, p = d === void 0 ? !1 : d, h = e.id, g = e.inputComponent, m = g === void 0 ? "input" : g, y = e.inputProps, w = y === void 0 ? {} : y, k = e.inputRef;
  e.margin;
  var j = e.multiline, f = j === void 0 ? !1 : j, x = e.name, E = e.onBlur, L = e.onChange, W = e.onClick, O = e.onFocus, P = e.onKeyDown, A = e.onKeyUp, v = e.placeholder, C = e.readOnly, M = e.renderSuffix, D = e.rows, $ = e.rowsMax, H = e.rowsMin, F = e.maxRows, q = e.minRows, z = e.startAdornment, Z = e.type, Q = Z === void 0 ? "text" : Z, ue = e.value, de = Ae(e, ["aria-describedby", "autoComplete", "autoFocus", "classes", "className", "color", "defaultValue", "disabled", "endAdornment", "error", "fullWidth", "id", "inputComponent", "inputProps", "inputRef", "margin", "multiline", "name", "onBlur", "onChange", "onClick", "onFocus", "onKeyDown", "onKeyUp", "placeholder", "readOnly", "renderSuffix", "rows", "rowsMax", "rowsMin", "maxRows", "minRows", "startAdornment", "type", "value"]), Se = w.value != null ? w.value : ue, fe = b.useRef(Se != null), ye = fe.current, be = b.useRef(), we = b.useCallback(function(te) {
  }, []), Ee = St(w.ref, we), ge = St(k, Ee), ke = St(be, ge), je = b.useState(!1), Te = je[0], He = je[1], _e = lu(), Fe = to({
    props: e,
    muiFormControl: _e,
    states: ["color", "disabled", "error", "hiddenLabel", "margin", "required", "filled"]
  });
  Fe.focused = _e ? _e.focused : Te, b.useEffect(function() {
    !_e && u && Te && (He(!1), E && E());
  }, [_e, u, Te, E]);
  var We = _e && _e.onFilled, Ue = _e && _e.onEmpty, tt = b.useCallback(function(te) {
    ls(te) ? We && We() : Ue && Ue();
  }, [We, Ue]);
  a0(function() {
    ye && tt({
      value: Se
    });
  }, [Se, tt, ye]);
  var Qe = function(oe) {
    if (Fe.disabled) {
      oe.stopPropagation();
      return;
    }
    O && O(oe), w.onFocus && w.onFocus(oe), _e && _e.onFocus ? _e.onFocus(oe) : He(!0);
  }, Ve = function(oe) {
    E && E(oe), w.onBlur && w.onBlur(oe), _e && _e.onBlur ? _e.onBlur(oe) : He(!1);
  }, ce = function(oe) {
    if (!ye) {
      var xe = oe.target || be.current;
      if (xe == null)
        throw new Error(vn(1));
      tt({
        value: xe.value
      });
    }
    for (var Me = arguments.length, Ce = new Array(Me > 1 ? Me - 1 : 0), me = 1; me < Me; me++)
      Ce[me - 1] = arguments[me];
    w.onChange && w.onChange.apply(w, [oe].concat(Ce)), L && L.apply(void 0, [oe].concat(Ce));
  };
  b.useEffect(function() {
    tt(be.current);
  }, []);
  var pe = function(oe) {
    be.current && oe.currentTarget === oe.target && be.current.focus(), W && W(oe);
  }, X = m, V = Y({}, w, {
    ref: ke
  });
  typeof X != "string" ? V = Y({
    // Rename ref to inputRef as we don't know the
    // provided `inputComponent` structure.
    inputRef: ke,
    type: Q
  }, V, {
    ref: null
  }) : f ? D && !F && !q && !$ && !H ? X = "textarea" : (V = Y({
    minRows: D || q,
    rowsMax: $,
    maxRows: F
  }, V), X = t0) : V = Y({
    type: Q
  }, V);
  var U = function(oe) {
    tt(oe.animationName === "mui-auto-fill-cancel" ? be.current : {
      value: "x"
    });
  };
  return b.useEffect(function() {
    _e && _e.setAdornedStart(!!z);
  }, [_e, z]), /* @__PURE__ */ b.createElement("div", Y({
    className: he(i.root, i["color".concat(pt(Fe.color || "primary"))], s, Fe.disabled && i.disabled, Fe.error && i.error, p && i.fullWidth, Fe.focused && i.focused, _e && i.formControl, f && i.multiline, z && i.adornedStart, c && i.adornedEnd, Fe.margin === "dense" && i.marginDense),
    onClick: pe,
    ref: a
  }, de), z, /* @__PURE__ */ b.createElement(aa.Provider, {
    value: null
  }, /* @__PURE__ */ b.createElement(X, Y({
    "aria-invalid": Fe.error,
    "aria-describedby": r,
    autoComplete: n,
    autoFocus: o,
    defaultValue: l,
    disabled: Fe.disabled,
    id: h,
    onAnimationStart: U,
    name: x,
    placeholder: v,
    readOnly: C,
    required: Fe.required,
    rows: D,
    value: Se,
    onKeyDown: P,
    onKeyUp: A
  }, V, {
    className: he(i.input, w.className, Fe.disabled && i.disabled, f && i.inputMultiline, Fe.hiddenLabel && i.inputHiddenLabel, z && i.inputAdornedStart, c && i.inputAdornedEnd, Q === "search" && i.inputTypeSearch, Fe.margin === "dense" && i.inputMarginDense),
    onBlur: Ve,
    onChange: ce,
    onFocus: Qe
  }))), c, M ? M(Y({}, Fe, {
    startAdornment: z
  })) : null);
});
const ro = et(n0, {
  name: "MuiInputBase"
})(o0);
var i0 = function(e) {
  var a = e.palette.type === "light", r = a ? "rgba(0, 0, 0, 0.42)" : "rgba(255, 255, 255, 0.7)", n = a ? "rgba(0, 0, 0, 0.09)" : "rgba(255, 255, 255, 0.09)";
  return {
    /* Styles applied to the root element. */
    root: {
      position: "relative",
      backgroundColor: n,
      borderTopLeftRadius: e.shape.borderRadius,
      borderTopRightRadius: e.shape.borderRadius,
      transition: e.transitions.create("background-color", {
        duration: e.transitions.duration.shorter,
        easing: e.transitions.easing.easeOut
      }),
      "&:hover": {
        backgroundColor: a ? "rgba(0, 0, 0, 0.13)" : "rgba(255, 255, 255, 0.13)",
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: n
        }
      },
      "&$focused": {
        backgroundColor: a ? "rgba(0, 0, 0, 0.09)" : "rgba(255, 255, 255, 0.09)"
      },
      "&$disabled": {
        backgroundColor: a ? "rgba(0, 0, 0, 0.12)" : "rgba(255, 255, 255, 0.12)"
      }
    },
    /* Styles applied to the root element if color secondary. */
    colorSecondary: {
      "&$underline:after": {
        borderBottomColor: e.palette.secondary.main
      }
    },
    /* Styles applied to the root element if `disableUnderline={false}`. */
    underline: {
      "&:after": {
        borderBottom: "2px solid ".concat(e.palette.primary.main),
        left: 0,
        bottom: 0,
        // Doing the other way around crash on IE 11 "''" https://github.com/cssinjs/jss/issues/242
        content: '""',
        position: "absolute",
        right: 0,
        transform: "scaleX(0)",
        transition: e.transitions.create("transform", {
          duration: e.transitions.duration.shorter,
          easing: e.transitions.easing.easeOut
        }),
        pointerEvents: "none"
        // Transparent to the hover style.
      },
      "&$focused:after": {
        transform: "scaleX(1)"
      },
      "&$error:after": {
        borderBottomColor: e.palette.error.main,
        transform: "scaleX(1)"
        // error is always underlined in red
      },
      "&:before": {
        borderBottom: "1px solid ".concat(r),
        left: 0,
        bottom: 0,
        // Doing the other way around crash on IE 11 "''" https://github.com/cssinjs/jss/issues/242
        content: '"\\00a0"',
        position: "absolute",
        right: 0,
        transition: e.transitions.create("border-bottom-color", {
          duration: e.transitions.duration.shorter
        }),
        pointerEvents: "none"
        // Transparent to the hover style.
      },
      "&:hover:before": {
        borderBottom: "1px solid ".concat(e.palette.text.primary)
      },
      "&$disabled:before": {
        borderBottomStyle: "dotted"
      }
    },
    /* Pseudo-class applied to the root element if the component is focused. */
    focused: {},
    /* Pseudo-class applied to the root element if `disabled={true}`. */
    disabled: {},
    /* Styles applied to the root element if `startAdornment` is provided. */
    adornedStart: {
      paddingLeft: 12
    },
    /* Styles applied to the root element if `endAdornment` is provided. */
    adornedEnd: {
      paddingRight: 12
    },
    /* Pseudo-class applied to the root element if `error={true}`. */
    error: {},
    /* Styles applied to the `input` element if `margin="dense"`. */
    marginDense: {},
    /* Styles applied to the root element if `multiline={true}`. */
    multiline: {
      padding: "27px 12px 10px",
      "&$marginDense": {
        paddingTop: 23,
        paddingBottom: 6
      }
    },
    /* Styles applied to the `input` element. */
    input: {
      padding: "27px 12px 10px",
      "&:-webkit-autofill": {
        WebkitBoxShadow: e.palette.type === "light" ? null : "0 0 0 100px #266798 inset",
        WebkitTextFillColor: e.palette.type === "light" ? null : "#fff",
        caretColor: e.palette.type === "light" ? null : "#fff",
        borderTopLeftRadius: "inherit",
        borderTopRightRadius: "inherit"
      }
    },
    /* Styles applied to the `input` element if `margin="dense"`. */
    inputMarginDense: {
      paddingTop: 23,
      paddingBottom: 6
    },
    /* Styles applied to the `input` if in `<FormControl hiddenLabel />`. */
    inputHiddenLabel: {
      paddingTop: 18,
      paddingBottom: 19,
      "&$inputMarginDense": {
        paddingTop: 10,
        paddingBottom: 11
      }
    },
    /* Styles applied to the `input` element if `multiline={true}`. */
    inputMultiline: {
      padding: 0
    },
    /* Styles applied to the `input` element if `startAdornment` is provided. */
    inputAdornedStart: {
      paddingLeft: 0
    },
    /* Styles applied to the `input` element if `endAdornment` is provided. */
    inputAdornedEnd: {
      paddingRight: 0
    }
  };
}, du = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.disableUnderline, n = e.classes, o = e.fullWidth, i = o === void 0 ? !1 : o, s = e.inputComponent, l = s === void 0 ? "input" : s, u = e.multiline, c = u === void 0 ? !1 : u, d = e.type, p = d === void 0 ? "text" : d, h = Ae(e, ["disableUnderline", "classes", "fullWidth", "inputComponent", "multiline", "type"]);
  return /* @__PURE__ */ b.createElement(ro, Y({
    classes: Y({}, n, {
      root: he(n.root, !r && n.underline),
      underline: null
    }),
    fullWidth: i,
    inputComponent: l,
    multiline: c,
    ref: a,
    type: p
  }, h));
});
du.muiName = "Input";
const s0 = et(i0, {
  name: "MuiFilledInput"
})(du);
var l0 = {
  /* Styles applied to the root element. */
  root: {
    display: "inline-flex",
    flexDirection: "column",
    position: "relative",
    // Reset fieldset default style.
    minWidth: 0,
    padding: 0,
    margin: 0,
    border: 0,
    verticalAlign: "top"
    // Fix alignment issue on Safari.
  },
  /* Styles applied to the root element if `margin="normal"`. */
  marginNormal: {
    marginTop: 16,
    marginBottom: 8
  },
  /* Styles applied to the root element if `margin="dense"`. */
  marginDense: {
    marginTop: 8,
    marginBottom: 4
  },
  /* Styles applied to the root element if `fullWidth={true}`. */
  fullWidth: {
    width: "100%"
  }
}, c0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.classes, o = e.className, i = e.color, s = i === void 0 ? "primary" : i, l = e.component, u = l === void 0 ? "div" : l, c = e.disabled, d = c === void 0 ? !1 : c, p = e.error, h = p === void 0 ? !1 : p, g = e.fullWidth, m = g === void 0 ? !1 : g, y = e.focused, w = e.hiddenLabel, k = w === void 0 ? !1 : w, j = e.margin, f = j === void 0 ? "none" : j, x = e.required, E = x === void 0 ? !1 : x, L = e.size, W = e.variant, O = W === void 0 ? "standard" : W, P = Ae(e, ["children", "classes", "className", "color", "component", "disabled", "error", "fullWidth", "focused", "hiddenLabel", "margin", "required", "size", "variant"]), A = b.useState(function() {
    var Se = !1;
    return r && b.Children.forEach(r, function(fe) {
      if (Ta(fe, ["Input", "Select"])) {
        var ye = Ta(fe, ["Select"]) ? fe.props.input : fe;
        ye && r0(ye.props) && (Se = !0);
      }
    }), Se;
  }), v = A[0], C = A[1], M = b.useState(function() {
    var Se = !1;
    return r && b.Children.forEach(r, function(fe) {
      Ta(fe, ["Input", "Select"]) && ls(fe.props, !0) && (Se = !0);
    }), Se;
  }), D = M[0], $ = M[1], H = b.useState(!1), F = H[0], q = H[1], z = y !== void 0 ? y : F;
  d && z && q(!1);
  var Z, Q = b.useCallback(function() {
    $(!0);
  }, []), ue = b.useCallback(function() {
    $(!1);
  }, []), de = {
    adornedStart: v,
    setAdornedStart: C,
    color: s,
    disabled: d,
    error: h,
    filled: D,
    focused: z,
    fullWidth: m,
    hiddenLabel: k,
    margin: (L === "small" ? "dense" : void 0) || f,
    onBlur: function() {
      q(!1);
    },
    onEmpty: ue,
    onFilled: Q,
    onFocus: function() {
      q(!0);
    },
    registerEffect: Z,
    required: E,
    variant: O
  };
  return /* @__PURE__ */ b.createElement(aa.Provider, {
    value: de
  }, /* @__PURE__ */ b.createElement(u, Y({
    className: he(n.root, o, f !== "none" && n["margin".concat(pt(f))], m && n.fullWidth),
    ref: a
  }, P), r));
});
const wl = et(l0, {
  name: "MuiFormControl"
})(c0);
var u0 = function(e) {
  return {
    /* Styles applied to the root element. */
    root: Y({
      color: e.palette.text.secondary
    }, e.typography.caption, {
      textAlign: "left",
      marginTop: 3,
      margin: 0,
      "&$disabled": {
        color: e.palette.text.disabled
      },
      "&$error": {
        color: e.palette.error.main
      }
    }),
    /* Pseudo-class applied to the root element if `error={true}`. */
    error: {},
    /* Pseudo-class applied to the root element if `disabled={true}`. */
    disabled: {},
    /* Styles applied to the root element if `margin="dense"`. */
    marginDense: {
      marginTop: 4
    },
    /* Styles applied to the root element if `variant="filled"` or `variant="outlined"`. */
    contained: {
      marginLeft: 14,
      marginRight: 14
    },
    /* Pseudo-class applied to the root element if `focused={true}`. */
    focused: {},
    /* Pseudo-class applied to the root element if `filled={true}`. */
    filled: {},
    /* Pseudo-class applied to the root element if `required={true}`. */
    required: {}
  };
}, d0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.classes, o = e.className, i = e.component, s = i === void 0 ? "p" : i;
  e.disabled, e.error, e.filled, e.focused, e.margin, e.required, e.variant;
  var l = Ae(e, ["children", "classes", "className", "component", "disabled", "error", "filled", "focused", "margin", "required", "variant"]), u = ss(), c = to({
    props: e,
    muiFormControl: u,
    states: ["variant", "margin", "disabled", "error", "filled", "focused", "required"]
  });
  return /* @__PURE__ */ b.createElement(s, Y({
    className: he(n.root, (c.variant === "filled" || c.variant === "outlined") && n.contained, o, c.disabled && n.disabled, c.error && n.error, c.filled && n.filled, c.focused && n.focused, c.required && n.required, c.margin === "dense" && n.marginDense),
    ref: a
  }, l), r === " " ? (
    // eslint-disable-next-line react/no-danger
    /* @__PURE__ */ b.createElement("span", {
      dangerouslySetInnerHTML: {
        __html: "&#8203;"
      }
    })
  ) : r);
});
const El = et(u0, {
  name: "MuiFormHelperText"
})(d0);
var f0 = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10], p0 = ["auto", !0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12];
function h0(t, e, a) {
  var r = {};
  p0.forEach(function(n) {
    var o = "grid-".concat(a, "-").concat(n);
    if (n === !0) {
      r[o] = {
        flexBasis: 0,
        flexGrow: 1,
        maxWidth: "100%"
      };
      return;
    }
    if (n === "auto") {
      r[o] = {
        flexBasis: "auto",
        flexGrow: 0,
        maxWidth: "none"
      };
      return;
    }
    var i = "".concat(Math.round(n / 12 * 1e8) / 1e6, "%");
    r[o] = {
      flexBasis: i,
      flexGrow: 0,
      maxWidth: i
    };
  }), a === "xs" ? Y(t, r) : t[e.breakpoints.up(a)] = r;
}
function Qo(t) {
  var e = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : 1, a = parseFloat(t);
  return "".concat(a / e).concat(String(t).replace(String(a), "") || "px");
}
function m0(t, e) {
  var a = {};
  return f0.forEach(function(r) {
    var n = t.spacing(r);
    n !== 0 && (a["spacing-".concat(e, "-").concat(r)] = {
      margin: "-".concat(Qo(n, 2)),
      width: "calc(100% + ".concat(Qo(n), ")"),
      "& > $item": {
        padding: Qo(n, 2)
      }
    });
  }), a;
}
var g0 = function(e) {
  return Y({
    /* Styles applied to the root element. */
    root: {},
    /* Styles applied to the root element if `container={true}`. */
    container: {
      boxSizing: "border-box",
      display: "flex",
      flexWrap: "wrap",
      width: "100%"
    },
    /* Styles applied to the root element if `item={true}`. */
    item: {
      boxSizing: "border-box",
      margin: "0"
      // For instance, it's useful when used with a `figure` element.
    },
    /* Styles applied to the root element if `zeroMinWidth={true}`. */
    zeroMinWidth: {
      minWidth: 0
    },
    /* Styles applied to the root element if `direction="column"`. */
    "direction-xs-column": {
      flexDirection: "column"
    },
    /* Styles applied to the root element if `direction="column-reverse"`. */
    "direction-xs-column-reverse": {
      flexDirection: "column-reverse"
    },
    /* Styles applied to the root element if `direction="row-reverse"`. */
    "direction-xs-row-reverse": {
      flexDirection: "row-reverse"
    },
    /* Styles applied to the root element if `wrap="nowrap"`. */
    "wrap-xs-nowrap": {
      flexWrap: "nowrap"
    },
    /* Styles applied to the root element if `wrap="reverse"`. */
    "wrap-xs-wrap-reverse": {
      flexWrap: "wrap-reverse"
    },
    /* Styles applied to the root element if `alignItems="center"`. */
    "align-items-xs-center": {
      alignItems: "center"
    },
    /* Styles applied to the root element if `alignItems="flex-start"`. */
    "align-items-xs-flex-start": {
      alignItems: "flex-start"
    },
    /* Styles applied to the root element if `alignItems="flex-end"`. */
    "align-items-xs-flex-end": {
      alignItems: "flex-end"
    },
    /* Styles applied to the root element if `alignItems="baseline"`. */
    "align-items-xs-baseline": {
      alignItems: "baseline"
    },
    /* Styles applied to the root element if `alignContent="center"`. */
    "align-content-xs-center": {
      alignContent: "center"
    },
    /* Styles applied to the root element if `alignContent="flex-start"`. */
    "align-content-xs-flex-start": {
      alignContent: "flex-start"
    },
    /* Styles applied to the root element if `alignContent="flex-end"`. */
    "align-content-xs-flex-end": {
      alignContent: "flex-end"
    },
    /* Styles applied to the root element if `alignContent="space-between"`. */
    "align-content-xs-space-between": {
      alignContent: "space-between"
    },
    /* Styles applied to the root element if `alignContent="space-around"`. */
    "align-content-xs-space-around": {
      alignContent: "space-around"
    },
    /* Styles applied to the root element if `justifyContent="center"`. */
    "justify-content-xs-center": {
      justifyContent: "center"
    },
    /* Styles applied to the root element if `justifyContent="flex-end"`. */
    "justify-content-xs-flex-end": {
      justifyContent: "flex-end"
    },
    /* Styles applied to the root element if `justifyContent="space-between"`. */
    "justify-content-xs-space-between": {
      justifyContent: "space-between"
    },
    /* Styles applied to the root element if `justifyContent="space-around"`. */
    "justify-content-xs-space-around": {
      justifyContent: "space-around"
    },
    /* Styles applied to the root element if `justifyContent="space-evenly"`. */
    "justify-content-xs-space-evenly": {
      justifyContent: "space-evenly"
    }
  }, m0(e, "xs"), e.breakpoints.keys.reduce(function(a, r) {
    return h0(a, e, r), a;
  }, {}));
}, v0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.alignContent, n = r === void 0 ? "stretch" : r, o = e.alignItems, i = o === void 0 ? "stretch" : o, s = e.classes, l = e.className, u = e.component, c = u === void 0 ? "div" : u, d = e.container, p = d === void 0 ? !1 : d, h = e.direction, g = h === void 0 ? "row" : h, m = e.item, y = m === void 0 ? !1 : m, w = e.justify, k = e.justifyContent, j = k === void 0 ? "flex-start" : k, f = e.lg, x = f === void 0 ? !1 : f, E = e.md, L = E === void 0 ? !1 : E, W = e.sm, O = W === void 0 ? !1 : W, P = e.spacing, A = P === void 0 ? 0 : P, v = e.wrap, C = v === void 0 ? "wrap" : v, M = e.xl, D = M === void 0 ? !1 : M, $ = e.xs, H = $ === void 0 ? !1 : $, F = e.zeroMinWidth, q = F === void 0 ? !1 : F, z = Ae(e, ["alignContent", "alignItems", "classes", "className", "component", "container", "direction", "item", "justify", "justifyContent", "lg", "md", "sm", "spacing", "wrap", "xl", "xs", "zeroMinWidth"]), Z = he(s.root, l, p && [s.container, A !== 0 && s["spacing-xs-".concat(String(A))]], y && s.item, q && s.zeroMinWidth, g !== "row" && s["direction-xs-".concat(String(g))], C !== "wrap" && s["wrap-xs-".concat(String(C))], i !== "stretch" && s["align-items-xs-".concat(String(i))], n !== "stretch" && s["align-content-xs-".concat(String(n))], (w || j) !== "flex-start" && s["justify-content-xs-".concat(String(w || j))], H !== !1 && s["grid-xs-".concat(String(H))], O !== !1 && s["grid-sm-".concat(String(O))], L !== !1 && s["grid-md-".concat(String(L))], x !== !1 && s["grid-lg-".concat(String(x))], D !== !1 && s["grid-xl-".concat(String(D))]);
  return /* @__PURE__ */ b.createElement(c, Y({
    className: Z,
    ref: a
  }, z));
}), kl = et(g0, {
  name: "MuiGrid"
})(v0);
function Pi(t) {
  return "scale(".concat(t, ", ").concat(Math.pow(t, 2), ")");
}
var b0 = {
  entering: {
    opacity: 1,
    transform: Pi(1)
  },
  entered: {
    opacity: 1,
    transform: "none"
  }
}, cs = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.disableStrictModeCompat, o = n === void 0 ? !1 : n, i = e.in, s = e.onEnter, l = e.onEntered, u = e.onEntering, c = e.onExit, d = e.onExited, p = e.onExiting, h = e.style, g = e.timeout, m = g === void 0 ? "auto" : g, y = e.TransitionComponent, w = y === void 0 ? or : y, k = Ae(e, ["children", "disableStrictModeCompat", "in", "onEnter", "onEntered", "onEntering", "onExit", "onExited", "onExiting", "style", "timeout", "TransitionComponent"]), j = b.useRef(), f = b.useRef(), x = $n(), E = x.unstable_strictMode && !o, L = b.useRef(null), W = St(r.ref, a), O = St(E ? L : void 0, W), P = function(q) {
    return function(z, Z) {
      if (q) {
        var Q = E ? [L.current, z] : [z, Z], ue = En(Q, 2), de = ue[0], Se = ue[1];
        Se === void 0 ? q(de) : q(de, Se);
      }
    };
  }, A = P(u), v = P(function(F, q) {
    rg(F);
    var z = Va({
      style: h,
      timeout: m
    }, {
      mode: "enter"
    }), Z = z.duration, Q = z.delay, ue;
    m === "auto" ? (ue = x.transitions.getAutoHeightDuration(F.clientHeight), f.current = ue) : ue = Z, F.style.transition = [x.transitions.create("opacity", {
      duration: ue,
      delay: Q
    }), x.transitions.create("transform", {
      duration: ue * 0.666,
      delay: Q
    })].join(","), s && s(F, q);
  }), C = P(l), M = P(p), D = P(function(F) {
    var q = Va({
      style: h,
      timeout: m
    }, {
      mode: "exit"
    }), z = q.duration, Z = q.delay, Q;
    m === "auto" ? (Q = x.transitions.getAutoHeightDuration(F.clientHeight), f.current = Q) : Q = z, F.style.transition = [x.transitions.create("opacity", {
      duration: Q,
      delay: Z
    }), x.transitions.create("transform", {
      duration: Q * 0.666,
      delay: Z || Q * 0.333
    })].join(","), F.style.opacity = "0", F.style.transform = Pi(0.75), c && c(F);
  }), $ = P(d), H = function(q, z) {
    var Z = E ? q : z;
    m === "auto" && (j.current = setTimeout(Z, f.current || 0));
  };
  return b.useEffect(function() {
    return function() {
      clearTimeout(j.current);
    };
  }, []), /* @__PURE__ */ b.createElement(w, Y({
    appear: !0,
    in: i,
    nodeRef: E ? L : void 0,
    onEnter: v,
    onEntered: C,
    onEntering: A,
    onExit: D,
    onExited: $,
    onExiting: M,
    addEndListener: H,
    timeout: m === "auto" ? null : m
  }, k), function(F, q) {
    return /* @__PURE__ */ b.cloneElement(r, Y({
      style: Y({
        opacity: 0,
        transform: Pi(0.75),
        visibility: F === "exited" && !i ? "hidden" : void 0
      }, b0[F], h, r.props.style),
      ref: O
    }, q));
  });
});
cs.muiSupportAuto = !0;
var y0 = function(e) {
  var a = e.palette.type === "light", r = a ? "rgba(0, 0, 0, 0.42)" : "rgba(255, 255, 255, 0.7)";
  return {
    /* Styles applied to the root element. */
    root: {
      position: "relative"
    },
    /* Styles applied to the root element if the component is a descendant of `FormControl`. */
    formControl: {
      "label + &": {
        marginTop: 16
      }
    },
    /* Styles applied to the root element if the component is focused. */
    focused: {},
    /* Styles applied to the root element if `disabled={true}`. */
    disabled: {},
    /* Styles applied to the root element if color secondary. */
    colorSecondary: {
      "&$underline:after": {
        borderBottomColor: e.palette.secondary.main
      }
    },
    /* Styles applied to the root element if `disableUnderline={false}`. */
    underline: {
      "&:after": {
        borderBottom: "2px solid ".concat(e.palette.primary.main),
        left: 0,
        bottom: 0,
        // Doing the other way around crash on IE 11 "''" https://github.com/cssinjs/jss/issues/242
        content: '""',
        position: "absolute",
        right: 0,
        transform: "scaleX(0)",
        transition: e.transitions.create("transform", {
          duration: e.transitions.duration.shorter,
          easing: e.transitions.easing.easeOut
        }),
        pointerEvents: "none"
        // Transparent to the hover style.
      },
      "&$focused:after": {
        transform: "scaleX(1)"
      },
      "&$error:after": {
        borderBottomColor: e.palette.error.main,
        transform: "scaleX(1)"
        // error is always underlined in red
      },
      "&:before": {
        borderBottom: "1px solid ".concat(r),
        left: 0,
        bottom: 0,
        // Doing the other way around crash on IE 11 "''" https://github.com/cssinjs/jss/issues/242
        content: '"\\00a0"',
        position: "absolute",
        right: 0,
        transition: e.transitions.create("border-bottom-color", {
          duration: e.transitions.duration.shorter
        }),
        pointerEvents: "none"
        // Transparent to the hover style.
      },
      "&:hover:not($disabled):before": {
        borderBottom: "2px solid ".concat(e.palette.text.primary),
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          borderBottom: "1px solid ".concat(r)
        }
      },
      "&$disabled:before": {
        borderBottomStyle: "dotted"
      }
    },
    /* Pseudo-class applied to the root element if `error={true}`. */
    error: {},
    /* Styles applied to the `input` element if `margin="dense"`. */
    marginDense: {},
    /* Styles applied to the root element if `multiline={true}`. */
    multiline: {},
    /* Styles applied to the root element if `fullWidth={true}`. */
    fullWidth: {},
    /* Styles applied to the `input` element. */
    input: {},
    /* Styles applied to the `input` element if `margin="dense"`. */
    inputMarginDense: {},
    /* Styles applied to the `input` element if `multiline={true}`. */
    inputMultiline: {},
    /* Styles applied to the `input` element if `type="search"`. */
    inputTypeSearch: {}
  };
}, fu = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.disableUnderline, n = e.classes, o = e.fullWidth, i = o === void 0 ? !1 : o, s = e.inputComponent, l = s === void 0 ? "input" : s, u = e.multiline, c = u === void 0 ? !1 : u, d = e.type, p = d === void 0 ? "text" : d, h = Ae(e, ["disableUnderline", "classes", "fullWidth", "inputComponent", "multiline", "type"]);
  return /* @__PURE__ */ b.createElement(ro, Y({
    classes: Y({}, n, {
      root: he(n.root, !r && n.underline),
      underline: null
    }),
    fullWidth: i,
    inputComponent: l,
    multiline: c,
    ref: a,
    type: p
  }, h));
});
fu.muiName = "Input";
const pu = et(y0, {
  name: "MuiInput"
})(fu);
var x0 = {
  /* Styles applied to the root element. */
  root: {
    display: "flex",
    height: "0.01em",
    // Fix IE 11 flexbox alignment. To remove at some point.
    maxHeight: "2em",
    alignItems: "center",
    whiteSpace: "nowrap"
  },
  /* Styles applied to the root element if `variant="filled"`. */
  filled: {
    "&$positionStart:not($hiddenLabel)": {
      marginTop: 16
    }
  },
  /* Styles applied to the root element if `position="start"`. */
  positionStart: {
    marginRight: 8
  },
  /* Styles applied to the root element if `position="end"`. */
  positionEnd: {
    marginLeft: 8
  },
  /* Styles applied to the root element if `disablePointerEvents=true`. */
  disablePointerEvents: {
    pointerEvents: "none"
  },
  /* Styles applied if the adornment is used inside <FormControl hiddenLabel />. */
  hiddenLabel: {},
  /* Styles applied if the adornment is used inside <FormControl margin="dense" />. */
  marginDense: {}
}, C0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.classes, o = e.className, i = e.component, s = i === void 0 ? "div" : i, l = e.disablePointerEvents, u = l === void 0 ? !1 : l, c = e.disableTypography, d = c === void 0 ? !1 : c, p = e.position, h = e.variant, g = Ae(e, ["children", "classes", "className", "component", "disablePointerEvents", "disableTypography", "position", "variant"]), m = lu() || {}, y = h;
  return h && m.variant, m && !y && (y = m.variant), /* @__PURE__ */ b.createElement(aa.Provider, {
    value: null
  }, /* @__PURE__ */ b.createElement(s, Y({
    className: he(n.root, o, p === "end" ? n.positionEnd : n.positionStart, u && n.disablePointerEvents, m.hiddenLabel && n.hiddenLabel, y === "filled" && n.filled, m.margin === "dense" && n.marginDense),
    ref: a
  }, g), typeof r == "string" && !d ? /* @__PURE__ */ b.createElement(Mt, {
    color: "textSecondary"
  }, r) : r));
});
const S0 = et(x0, {
  name: "MuiInputAdornment"
})(C0);
var w0 = {
  /* Styles applied to the root element. */
  root: {},
  /* Styles applied to the root element if `underline="none"`. */
  underlineNone: {
    textDecoration: "none"
  },
  /* Styles applied to the root element if `underline="hover"`. */
  underlineHover: {
    textDecoration: "none",
    "&:hover": {
      textDecoration: "underline"
    }
  },
  /* Styles applied to the root element if `underline="always"`. */
  underlineAlways: {
    textDecoration: "underline"
  },
  // Same reset as ButtonBase.root
  /* Styles applied to the root element if `component="button"`. */
  button: {
    position: "relative",
    WebkitTapHighlightColor: "transparent",
    backgroundColor: "transparent",
    // Reset default value
    // We disable the focus ring for mouse, touch and keyboard users.
    outline: 0,
    border: 0,
    margin: 0,
    // Remove the margin in Safari
    borderRadius: 0,
    padding: 0,
    // Remove the padding in Firefox
    cursor: "pointer",
    userSelect: "none",
    verticalAlign: "middle",
    "-moz-appearance": "none",
    // Reset
    "-webkit-appearance": "none",
    // Reset
    "&::-moz-focus-inner": {
      borderStyle: "none"
      // Remove Firefox dotted outline.
    },
    "&$focusVisible": {
      outline: "auto"
    }
  },
  /* Pseudo-class applied to the root element if the link is keyboard focused. */
  focusVisible: {}
}, E0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.classes, n = e.className, o = e.color, i = o === void 0 ? "primary" : o, s = e.component, l = s === void 0 ? "a" : s, u = e.onBlur, c = e.onFocus, d = e.TypographyClasses, p = e.underline, h = p === void 0 ? "hover" : p, g = e.variant, m = g === void 0 ? "inherit" : g, y = Ae(e, ["classes", "className", "color", "component", "onBlur", "onFocus", "TypographyClasses", "underline", "variant"]), w = as(), k = w.isFocusVisible, j = w.onBlurVisible, f = w.ref, x = b.useState(!1), E = x[0], L = x[1], W = St(a, f), O = function(v) {
    E && (j(), L(!1)), u && u(v);
  }, P = function(v) {
    k(v) && L(!0), c && c(v);
  };
  return /* @__PURE__ */ b.createElement(Mt, Y({
    className: he(r.root, r["underline".concat(pt(h))], n, E && r.focusVisible, l === "button" && r.button),
    classes: d,
    color: i,
    component: l,
    onBlur: O,
    onFocus: P,
    ref: W,
    variant: m
  }, y));
});
const k0 = et(w0, {
  name: "MuiLink"
})(E0);
var Oa = b.createContext({}), $0 = {
  /* Styles applied to the root element. */
  root: {
    listStyle: "none",
    margin: 0,
    padding: 0,
    position: "relative"
  },
  /* Styles applied to the root element if `disablePadding={false}`. */
  padding: {
    paddingTop: 8,
    paddingBottom: 8
  },
  /* Styles applied to the root element if dense. */
  dense: {},
  /* Styles applied to the root element if a `subheader` is provided. */
  subheader: {
    paddingTop: 0
  }
}, R0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.classes, o = e.className, i = e.component, s = i === void 0 ? "ul" : i, l = e.dense, u = l === void 0 ? !1 : l, c = e.disablePadding, d = c === void 0 ? !1 : c, p = e.subheader, h = Ae(e, ["children", "classes", "className", "component", "dense", "disablePadding", "subheader"]), g = b.useMemo(function() {
    return {
      dense: u
    };
  }, [u]);
  return /* @__PURE__ */ b.createElement(Oa.Provider, {
    value: g
  }, /* @__PURE__ */ b.createElement(s, Y({
    className: he(n.root, o, u && n.dense, !d && n.padding, p && n.subheader),
    ref: a
  }, h), p, r));
});
const T0 = et($0, {
  name: "MuiList"
})(R0);
var O0 = function(e) {
  return {
    /* Styles applied to the (normally root) `component` element. May be wrapped by a `container`. */
    root: {
      display: "flex",
      justifyContent: "flex-start",
      alignItems: "center",
      position: "relative",
      textDecoration: "none",
      width: "100%",
      boxSizing: "border-box",
      textAlign: "left",
      paddingTop: 8,
      paddingBottom: 8,
      "&$focusVisible": {
        backgroundColor: e.palette.action.selected
      },
      "&$selected, &$selected:hover": {
        backgroundColor: e.palette.action.selected
      },
      "&$disabled": {
        opacity: 0.5
      }
    },
    /* Styles applied to the `container` element if `children` includes `ListItemSecondaryAction`. */
    container: {
      position: "relative"
    },
    /* Pseudo-class applied to the `component`'s `focusVisibleClassName` prop if `button={true}`. */
    focusVisible: {},
    /* Styles applied to the `component` element if dense. */
    dense: {
      paddingTop: 4,
      paddingBottom: 4
    },
    /* Styles applied to the `component` element if `alignItems="flex-start"`. */
    alignItemsFlexStart: {
      alignItems: "flex-start"
    },
    /* Pseudo-class applied to the inner `component` element if `disabled={true}`. */
    disabled: {},
    /* Styles applied to the inner `component` element if `divider={true}`. */
    divider: {
      borderBottom: "1px solid ".concat(e.palette.divider),
      backgroundClip: "padding-box"
    },
    /* Styles applied to the inner `component` element if `disableGutters={false}`. */
    gutters: {
      paddingLeft: 16,
      paddingRight: 16
    },
    /* Styles applied to the inner `component` element if `button={true}`. */
    button: {
      transition: e.transitions.create("background-color", {
        duration: e.transitions.duration.shortest
      }),
      "&:hover": {
        textDecoration: "none",
        backgroundColor: e.palette.action.hover,
        // Reset on touch devices, it doesn't add specificity
        "@media (hover: none)": {
          backgroundColor: "transparent"
        }
      }
    },
    /* Styles applied to the `component` element if `children` includes `ListItemSecondaryAction`. */
    secondaryAction: {
      // Add some space to avoid collision as `ListItemSecondaryAction`
      // is absolutely positioned.
      paddingRight: 48
    },
    /* Pseudo-class applied to the root element if `selected={true}`. */
    selected: {}
  };
}, _0 = typeof window > "u" ? b.useEffect : b.useLayoutEffect, P0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.alignItems, n = r === void 0 ? "center" : r, o = e.autoFocus, i = o === void 0 ? !1 : o, s = e.button, l = s === void 0 ? !1 : s, u = e.children, c = e.classes, d = e.className, p = e.component, h = e.ContainerComponent, g = h === void 0 ? "li" : h, m = e.ContainerProps;
  m = m === void 0 ? {} : m;
  var y = m.className, w = Ae(m, ["className"]), k = e.dense, j = k === void 0 ? !1 : k, f = e.disabled, x = f === void 0 ? !1 : f, E = e.disableGutters, L = E === void 0 ? !1 : E, W = e.divider, O = W === void 0 ? !1 : W, P = e.focusVisibleClassName, A = e.selected, v = A === void 0 ? !1 : A, C = Ae(e, ["alignItems", "autoFocus", "button", "children", "classes", "className", "component", "ContainerComponent", "ContainerProps", "dense", "disabled", "disableGutters", "divider", "focusVisibleClassName", "selected"]), M = b.useContext(Oa), D = {
    dense: j || M.dense || !1,
    alignItems: n
  }, $ = b.useRef(null);
  _0(function() {
    i && $.current && $.current.focus();
  }, [i]);
  var H = b.Children.toArray(u), F = H.length && Ta(H[H.length - 1], ["ListItemSecondaryAction"]), q = b.useCallback(function(ue) {
    $.current = Ut.findDOMNode(ue);
  }, []), z = St(q, a), Z = Y({
    className: he(c.root, d, D.dense && c.dense, !L && c.gutters, O && c.divider, x && c.disabled, l && c.button, n !== "center" && c.alignItemsFlexStart, F && c.secondaryAction, v && c.selected),
    disabled: x
  }, C), Q = p || "li";
  return l && (Z.component = p || "div", Z.focusVisibleClassName = he(c.focusVisible, P), Q = kr), F ? (Q = !Z.component && !p ? "div" : Q, g === "li" && (Q === "li" ? Q = "div" : Z.component === "li" && (Z.component = "div")), /* @__PURE__ */ b.createElement(Oa.Provider, {
    value: D
  }, /* @__PURE__ */ b.createElement(g, Y({
    className: he(c.container, y),
    ref: z
  }, w), /* @__PURE__ */ b.createElement(Q, Z, H), H.pop()))) : /* @__PURE__ */ b.createElement(Oa.Provider, {
    value: D
  }, /* @__PURE__ */ b.createElement(Q, Y({
    ref: z
  }, Z), H));
});
const j0 = et(O0, {
  name: "MuiListItem"
})(P0);
function $l(t, e) {
  var a = 0;
  return typeof e == "number" ? a = e : e === "center" ? a = t.height / 2 : e === "bottom" && (a = t.height), a;
}
function Rl(t, e) {
  var a = 0;
  return typeof e == "number" ? a = e : e === "center" ? a = t.width / 2 : e === "right" && (a = t.width), a;
}
function Tl(t) {
  return [t.horizontal, t.vertical].map(function(e) {
    return typeof e == "number" ? "".concat(e, "px") : e;
  }).join(" ");
}
function I0(t, e) {
  for (var a = e, r = 0; a && a !== t; )
    a = a.parentElement, r += a.scrollTop;
  return r;
}
function ei(t) {
  return typeof t == "function" ? t() : t;
}
var A0 = {
  /* Styles applied to the root element. */
  root: {},
  /* Styles applied to the `Paper` component. */
  paper: {
    position: "absolute",
    overflowY: "auto",
    overflowX: "hidden",
    // So we see the popover when it's empty.
    // It's most likely on issue on userland.
    minWidth: 16,
    minHeight: 16,
    maxWidth: "calc(100% - 32px)",
    maxHeight: "calc(100% - 32px)",
    // We disable the focus ring for mouse, touch and keyboard users.
    outline: 0
  }
}, M0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.action, n = e.anchorEl, o = e.anchorOrigin, i = o === void 0 ? {
    vertical: "top",
    horizontal: "left"
  } : o, s = e.anchorPosition, l = e.anchorReference, u = l === void 0 ? "anchorEl" : l, c = e.children, d = e.classes, p = e.className, h = e.container, g = e.elevation, m = g === void 0 ? 8 : g, y = e.getContentAnchorEl, w = e.marginThreshold, k = w === void 0 ? 16 : w, j = e.onEnter, f = e.onEntered, x = e.onEntering, E = e.onExit, L = e.onExited, W = e.onExiting, O = e.open, P = e.PaperProps, A = P === void 0 ? {} : P, v = e.transformOrigin, C = v === void 0 ? {
    vertical: "top",
    horizontal: "left"
  } : v, M = e.TransitionComponent, D = M === void 0 ? cs : M, $ = e.transitionDuration, H = $ === void 0 ? "auto" : $, F = e.TransitionProps, q = F === void 0 ? {} : F, z = Ae(e, ["action", "anchorEl", "anchorOrigin", "anchorPosition", "anchorReference", "children", "classes", "className", "container", "elevation", "getContentAnchorEl", "marginThreshold", "onEnter", "onEntered", "onEntering", "onExit", "onExited", "onExiting", "open", "PaperProps", "transformOrigin", "TransitionComponent", "transitionDuration", "TransitionProps"]), Z = b.useRef(), Q = b.useCallback(function(ge) {
    if (u === "anchorPosition")
      return s;
    var ke = ei(n), je = ke && ke.nodeType === 1 ? ke : cr(Z.current).body, Te = je.getBoundingClientRect(), He = ge === 0 ? i.vertical : "center";
    return {
      top: Te.top + $l(Te, He),
      left: Te.left + Rl(Te, i.horizontal)
    };
  }, [n, i.horizontal, i.vertical, s, u]), ue = b.useCallback(function(ge) {
    var ke = 0;
    if (y && u === "anchorEl") {
      var je = y(ge);
      if (je && ge.contains(je)) {
        var Te = I0(ge, je);
        ke = je.offsetTop + je.clientHeight / 2 - Te || 0;
      }
    }
    return ke;
  }, [i.vertical, u, y]), de = b.useCallback(function(ge) {
    var ke = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : 0;
    return {
      vertical: $l(ge, C.vertical) + ke,
      horizontal: Rl(ge, C.horizontal)
    };
  }, [C.horizontal, C.vertical]), Se = b.useCallback(function(ge) {
    var ke = ue(ge), je = {
      width: ge.offsetWidth,
      height: ge.offsetHeight
    }, Te = de(je, ke);
    if (u === "none")
      return {
        top: null,
        left: null,
        transformOrigin: Tl(Te)
      };
    var He = Q(ke), _e = He.top - Te.vertical, Fe = He.left - Te.horizontal, We = _e + je.height, Ue = Fe + je.width, tt = rs(ei(n)), Qe = tt.innerHeight - k, Ve = tt.innerWidth - k;
    if (_e < k) {
      var ce = _e - k;
      _e -= ce, Te.vertical += ce;
    } else if (We > Qe) {
      var pe = We - Qe;
      _e -= pe, Te.vertical += pe;
    }
    if (Fe < k) {
      var X = Fe - k;
      Fe -= X, Te.horizontal += X;
    } else if (Ue > Ve) {
      var V = Ue - Ve;
      Fe -= V, Te.horizontal += V;
    }
    return {
      top: "".concat(Math.round(_e), "px"),
      left: "".concat(Math.round(Fe), "px"),
      transformOrigin: Tl(Te)
    };
  }, [n, u, Q, ue, de, k]), fe = b.useCallback(function() {
    var ge = Z.current;
    if (ge) {
      var ke = Se(ge);
      ke.top !== null && (ge.style.top = ke.top), ke.left !== null && (ge.style.left = ke.left), ge.style.transformOrigin = ke.transformOrigin;
    }
  }, [Se]), ye = function(ke, je) {
    x && x(ke, je), fe();
  }, be = b.useCallback(function(ge) {
    Z.current = Ut.findDOMNode(ge);
  }, []);
  b.useEffect(function() {
    O && fe();
  }), b.useImperativeHandle(r, function() {
    return O ? {
      updatePosition: function() {
        fe();
      }
    } : null;
  }, [O, fe]), b.useEffect(function() {
    if (O) {
      var ge = Yn(function() {
        fe();
      });
      return window.addEventListener("resize", ge), function() {
        ge.clear(), window.removeEventListener("resize", ge);
      };
    }
  }, [O, fe]);
  var we = H;
  H === "auto" && !D.muiSupportAuto && (we = void 0);
  var Ee = h || (n ? cr(ei(n)).body : void 0);
  return /* @__PURE__ */ b.createElement(Yg, Y({
    container: Ee,
    open: O,
    ref: a,
    BackdropProps: {
      invisible: !0
    },
    className: he(d.root, p)
  }, z), /* @__PURE__ */ b.createElement(D, Y({
    appear: !0,
    in: O,
    onEnter: j,
    onEntered: f,
    onExit: E,
    onExited: L,
    onExiting: W,
    timeout: we
  }, q, {
    onEntering: Gn(ye, q.onEntering)
  }), /* @__PURE__ */ b.createElement(ou, Y({
    elevation: m,
    ref: be
  }, A, {
    className: he(d.paper, A.className)
  }), c)));
});
const N0 = et(A0, {
  name: "MuiPopover"
})(M0);
function ti(t, e, a) {
  return t === e ? t.firstChild : e && e.nextElementSibling ? e.nextElementSibling : a ? null : t.firstChild;
}
function Ol(t, e, a) {
  return t === e ? a ? t.firstChild : t.lastChild : e && e.previousElementSibling ? e.previousElementSibling : a ? null : t.lastChild;
}
function hu(t, e) {
  if (e === void 0)
    return !0;
  var a = t.innerText;
  return a === void 0 && (a = t.textContent), a = a.trim().toLowerCase(), a.length === 0 ? !1 : e.repeating ? a[0] === e.keys[0] : a.indexOf(e.keys.join("")) === 0;
}
function In(t, e, a, r, n, o) {
  for (var i = !1, s = n(t, e, e ? a : !1); s; ) {
    if (s === t.firstChild) {
      if (i)
        return;
      i = !0;
    }
    var l = r ? !1 : s.disabled || s.getAttribute("aria-disabled") === "true";
    if (!s.hasAttribute("tabindex") || !hu(s, o) || l)
      s = n(t, s, a);
    else {
      s.focus();
      return;
    }
  }
}
var L0 = typeof window > "u" ? b.useEffect : b.useLayoutEffect, D0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.actions, n = e.autoFocus, o = n === void 0 ? !1 : n, i = e.autoFocusItem, s = i === void 0 ? !1 : i, l = e.children, u = e.className, c = e.disabledItemsFocusable, d = c === void 0 ? !1 : c, p = e.disableListWrap, h = p === void 0 ? !1 : p, g = e.onKeyDown, m = e.variant, y = m === void 0 ? "selectedMenu" : m, w = Ae(e, ["actions", "autoFocus", "autoFocusItem", "children", "className", "disabledItemsFocusable", "disableListWrap", "onKeyDown", "variant"]), k = b.useRef(null), j = b.useRef({
    keys: [],
    repeating: !0,
    previousKeyMatched: !0,
    lastTime: null
  });
  L0(function() {
    o && k.current.focus();
  }, [o]), b.useImperativeHandle(r, function() {
    return {
      adjustStyleForScrollbar: function(P, A) {
        var v = !k.current.style.width;
        if (P.clientHeight < k.current.clientHeight && v) {
          var C = "".concat(uu(), "px");
          k.current.style[A.direction === "rtl" ? "paddingLeft" : "paddingRight"] = C, k.current.style.width = "calc(100% + ".concat(C, ")");
        }
        return k.current;
      }
    };
  }, []);
  var f = function(P) {
    var A = k.current, v = P.key, C = cr(A).activeElement;
    if (v === "ArrowDown")
      P.preventDefault(), In(A, C, h, d, ti);
    else if (v === "ArrowUp")
      P.preventDefault(), In(A, C, h, d, Ol);
    else if (v === "Home")
      P.preventDefault(), In(A, null, h, d, ti);
    else if (v === "End")
      P.preventDefault(), In(A, null, h, d, Ol);
    else if (v.length === 1) {
      var M = j.current, D = v.toLowerCase(), $ = performance.now();
      M.keys.length > 0 && ($ - M.lastTime > 500 ? (M.keys = [], M.repeating = !0, M.previousKeyMatched = !0) : M.repeating && D !== M.keys[0] && (M.repeating = !1)), M.lastTime = $, M.keys.push(D);
      var H = C && !M.repeating && hu(C, M);
      M.previousKeyMatched && (H || In(A, C, !1, d, ti, M)) ? P.preventDefault() : M.previousKeyMatched = !1;
    }
    g && g(P);
  }, x = b.useCallback(function(O) {
    k.current = Ut.findDOMNode(O);
  }, []), E = St(x, a), L = -1;
  b.Children.forEach(l, function(O, P) {
    /* @__PURE__ */ b.isValidElement(O) && (O.props.disabled || (y === "selectedMenu" && O.props.selected || L === -1) && (L = P));
  });
  var W = b.Children.map(l, function(O, P) {
    if (P === L) {
      var A = {};
      return s && (A.autoFocus = !0), O.props.tabIndex === void 0 && y === "selectedMenu" && (A.tabIndex = 0), /* @__PURE__ */ b.cloneElement(O, A);
    }
    return O;
  });
  return /* @__PURE__ */ b.createElement(T0, Y({
    role: "menu",
    ref: E,
    className: u,
    onKeyDown: f,
    tabIndex: o ? 0 : -1
  }, w), W);
}), _l = {
  vertical: "top",
  horizontal: "right"
}, Pl = {
  vertical: "top",
  horizontal: "left"
}, B0 = {
  /* Styles applied to the `Paper` component. */
  paper: {
    // specZ: The maximum height of a simple menu should be one or more rows less than the view
    // height. This ensures a tapable area outside of the simple menu with which to dismiss
    // the menu.
    maxHeight: "calc(100% - 96px)",
    // Add iOS momentum scrolling.
    WebkitOverflowScrolling: "touch"
  },
  /* Styles applied to the `List` component via `MenuList`. */
  list: {
    // We disable the focus ring for mouse, touch and keyboard users.
    outline: 0
  }
}, F0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.autoFocus, n = r === void 0 ? !0 : r, o = e.children, i = e.classes, s = e.disableAutoFocusItem, l = s === void 0 ? !1 : s, u = e.MenuListProps, c = u === void 0 ? {} : u, d = e.onClose, p = e.onEntering, h = e.open, g = e.PaperProps, m = g === void 0 ? {} : g, y = e.PopoverClasses, w = e.transitionDuration, k = w === void 0 ? "auto" : w, j = e.TransitionProps;
  j = j === void 0 ? {} : j;
  var f = j.onEntering, x = Ae(j, ["onEntering"]), E = e.variant, L = E === void 0 ? "selectedMenu" : E, W = Ae(e, ["autoFocus", "children", "classes", "disableAutoFocusItem", "MenuListProps", "onClose", "onEntering", "open", "PaperProps", "PopoverClasses", "transitionDuration", "TransitionProps", "variant"]), O = $n(), P = n && !l && h, A = b.useRef(null), v = b.useRef(null), C = function() {
    return v.current;
  }, M = function(q, z) {
    A.current && A.current.adjustStyleForScrollbar(q, O), p && p(q, z), f && f(q, z);
  }, D = function(q) {
    q.key === "Tab" && (q.preventDefault(), d && d(q, "tabKeyDown"));
  }, $ = -1;
  b.Children.map(o, function(F, q) {
    /* @__PURE__ */ b.isValidElement(F) && (F.props.disabled || (L !== "menu" && F.props.selected || $ === -1) && ($ = q));
  });
  var H = b.Children.map(o, function(F, q) {
    return q === $ ? /* @__PURE__ */ b.cloneElement(F, {
      ref: function(Z) {
        v.current = Ut.findDOMNode(Z), Hr(F.ref, Z);
      }
    }) : F;
  });
  return /* @__PURE__ */ b.createElement(N0, Y({
    getContentAnchorEl: C,
    classes: y,
    onClose: d,
    TransitionProps: Y({
      onEntering: M
    }, x),
    anchorOrigin: O.direction === "rtl" ? _l : Pl,
    transformOrigin: O.direction === "rtl" ? _l : Pl,
    PaperProps: Y({}, m, {
      classes: Y({}, m.classes, {
        root: i.paper
      })
    }),
    open: h,
    ref: a,
    transitionDuration: k
  }, W), /* @__PURE__ */ b.createElement(D0, Y({
    onKeyDown: D,
    actions: A,
    autoFocus: n && ($ === -1 || l),
    autoFocusItem: P,
    variant: L
  }, c, {
    className: he(i.list, c.className)
  }), H));
});
const z0 = et(B0, {
  name: "MuiMenu"
})(F0);
var W0 = function(e) {
  return {
    /* Styles applied to the root element. */
    root: Y({}, e.typography.body1, Ot({
      minHeight: 48,
      paddingTop: 6,
      paddingBottom: 6,
      boxSizing: "border-box",
      width: "auto",
      overflow: "hidden",
      whiteSpace: "nowrap"
    }, e.breakpoints.up("sm"), {
      minHeight: "auto"
    })),
    // TODO v5: remove
    /* Styles applied to the root element if `disableGutters={false}`. */
    gutters: {},
    /* Styles applied to the root element if `selected={true}`. */
    selected: {},
    /* Styles applied to the root element if dense. */
    dense: Y({}, e.typography.body2, {
      minHeight: "auto"
    })
  };
}, V0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.classes, n = e.className, o = e.component, i = o === void 0 ? "li" : o, s = e.disableGutters, l = s === void 0 ? !1 : s, u = e.ListItemClasses, c = e.role, d = c === void 0 ? "menuitem" : c, p = e.selected, h = e.tabIndex, g = Ae(e, ["classes", "className", "component", "disableGutters", "ListItemClasses", "role", "selected", "tabIndex"]), m;
  return e.disabled || (m = h !== void 0 ? h : -1), /* @__PURE__ */ b.createElement(j0, Y({
    button: !0,
    role: d,
    tabIndex: m,
    component: i,
    selected: p,
    disableGutters: l,
    classes: Y({
      dense: r.dense
    }, u),
    className: he(r.root, n, p && r.selected, !l && r.gutters),
    ref: a
  }, g));
});
const xa = et(W0, {
  name: "MuiMenuItem"
})(V0);
var mu = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.classes, n = e.className, o = e.disabled, i = e.IconComponent, s = e.inputRef, l = e.variant, u = l === void 0 ? "standard" : l, c = Ae(e, ["classes", "className", "disabled", "IconComponent", "inputRef", "variant"]);
  return /* @__PURE__ */ b.createElement(b.Fragment, null, /* @__PURE__ */ b.createElement("select", Y({
    className: he(
      r.root,
      // TODO v5: merge root and select
      r.select,
      r[u],
      n,
      o && r.disabled
    ),
    disabled: o,
    ref: s || a
  }, c)), e.multiple ? null : /* @__PURE__ */ b.createElement(i, {
    className: he(r.icon, r["icon".concat(pt(u))], o && r.disabled)
  }));
});
const gu = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M7 10l5 5 5-5z"
}));
var vu = function(e) {
  return {
    /* Styles applied to the select component `root` class. */
    root: {},
    /* Styles applied to the select component `select` class. */
    select: {
      "-moz-appearance": "none",
      // Reset
      "-webkit-appearance": "none",
      // Reset
      // When interacting quickly, the text can end up selected.
      // Native select can't be selected either.
      userSelect: "none",
      borderRadius: 0,
      // Reset
      minWidth: 16,
      // So it doesn't collapse.
      cursor: "pointer",
      "&:focus": {
        // Show that it's not an text input
        backgroundColor: e.palette.type === "light" ? "rgba(0, 0, 0, 0.05)" : "rgba(255, 255, 255, 0.05)",
        borderRadius: 0
        // Reset Chrome style
      },
      // Remove IE 11 arrow
      "&::-ms-expand": {
        display: "none"
      },
      "&$disabled": {
        cursor: "default"
      },
      "&[multiple]": {
        height: "auto"
      },
      "&:not([multiple]) option, &:not([multiple]) optgroup": {
        backgroundColor: e.palette.background.paper
      },
      "&&": {
        paddingRight: 24
      }
    },
    /* Styles applied to the select component if `variant="filled"`. */
    filled: {
      "&&": {
        paddingRight: 32
      }
    },
    /* Styles applied to the select component if `variant="outlined"`. */
    outlined: {
      borderRadius: e.shape.borderRadius,
      "&&": {
        paddingRight: 32
      }
    },
    /* Styles applied to the select component `selectMenu` class. */
    selectMenu: {
      height: "auto",
      // Resets for multpile select with chips
      minHeight: "1.1876em",
      // Required for select\text-field height consistency
      textOverflow: "ellipsis",
      whiteSpace: "nowrap",
      overflow: "hidden"
    },
    /* Pseudo-class applied to the select component `disabled` class. */
    disabled: {},
    /* Styles applied to the icon component. */
    icon: {
      // We use a position absolute over a flexbox in order to forward the pointer events
      // to the input and to support wrapping tags..
      position: "absolute",
      right: 0,
      top: "calc(50% - 12px)",
      // Center vertically
      pointerEvents: "none",
      // Don't block pointer events on the select under the icon.
      color: e.palette.action.active,
      "&$disabled": {
        color: e.palette.action.disabled
      }
    },
    /* Styles applied to the icon component if the popup is open. */
    iconOpen: {
      transform: "rotate(180deg)"
    },
    /* Styles applied to the icon component if `variant="filled"`. */
    iconFilled: {
      right: 7
    },
    /* Styles applied to the icon component if `variant="outlined"`. */
    iconOutlined: {
      right: 7
    },
    /* Styles applied to the underlying native input component. */
    nativeInput: {
      bottom: 0,
      left: 0,
      position: "absolute",
      opacity: 0,
      pointerEvents: "none",
      width: "100%"
    }
  };
}, H0 = /* @__PURE__ */ b.createElement(pu, null), bu = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.children, n = e.classes, o = e.IconComponent, i = o === void 0 ? gu : o, s = e.input, l = s === void 0 ? H0 : s, u = e.inputProps;
  e.variant;
  var c = Ae(e, ["children", "classes", "IconComponent", "input", "inputProps", "variant"]), d = ss(), p = to({
    props: e,
    muiFormControl: d,
    states: ["variant"]
  });
  return /* @__PURE__ */ b.cloneElement(l, Y({
    // Most of the logic is implemented in `NativeSelectInput`.
    // The `Select` component is a simple API wrapper to expose something better to play with.
    inputComponent: mu,
    inputProps: Y({
      children: r,
      classes: n,
      IconComponent: i,
      variant: p.variant,
      type: void 0
    }, u, l ? l.props.inputProps : {}),
    ref: a
  }, c));
});
bu.muiName = "Select";
et(vu, {
  name: "MuiNativeSelect"
})(bu);
var K0 = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      position: "absolute",
      bottom: 0,
      right: 0,
      top: -5,
      left: 0,
      margin: 0,
      padding: "0 8px",
      pointerEvents: "none",
      borderRadius: "inherit",
      borderStyle: "solid",
      borderWidth: 1,
      overflow: "hidden"
    },
    /* Styles applied to the legend element when `labelWidth` is provided. */
    legend: {
      textAlign: "left",
      padding: 0,
      lineHeight: "11px",
      // sync with `height` in `legend` styles
      transition: e.transitions.create("width", {
        duration: 150,
        easing: e.transitions.easing.easeOut
      })
    },
    /* Styles applied to the legend element. */
    legendLabelled: {
      display: "block",
      width: "auto",
      textAlign: "left",
      padding: 0,
      height: 11,
      // sync with `lineHeight` in `legend` styles
      fontSize: "0.75em",
      visibility: "hidden",
      maxWidth: 0.01,
      transition: e.transitions.create("max-width", {
        duration: 50,
        easing: e.transitions.easing.easeOut
      }),
      "& > span": {
        paddingLeft: 5,
        paddingRight: 5,
        display: "inline-block"
      }
    },
    /* Styles applied to the legend element is notched. */
    legendNotched: {
      maxWidth: 1e3,
      transition: e.transitions.create("max-width", {
        duration: 100,
        easing: e.transitions.easing.easeOut,
        delay: 50
      })
    }
  };
}, U0 = /* @__PURE__ */ b.forwardRef(function(e, a) {
  e.children;
  var r = e.classes, n = e.className, o = e.label, i = e.labelWidth, s = e.notched, l = e.style, u = Ae(e, ["children", "classes", "className", "label", "labelWidth", "notched", "style"]), c = $n(), d = c.direction === "rtl" ? "right" : "left";
  if (o !== void 0)
    return /* @__PURE__ */ b.createElement("fieldset", Y({
      "aria-hidden": !0,
      className: he(r.root, n),
      ref: a,
      style: l
    }, u), /* @__PURE__ */ b.createElement("legend", {
      className: he(r.legendLabelled, s && r.legendNotched)
    }, o ? /* @__PURE__ */ b.createElement("span", null, o) : /* @__PURE__ */ b.createElement("span", {
      dangerouslySetInnerHTML: {
        __html: "&#8203;"
      }
    })));
  var p = i > 0 ? i * 0.75 + 8 : 0.01;
  return /* @__PURE__ */ b.createElement("fieldset", Y({
    "aria-hidden": !0,
    style: Y(Ot({}, "padding".concat(pt(d)), 8), l),
    className: he(r.root, n),
    ref: a
  }, u), /* @__PURE__ */ b.createElement("legend", {
    className: r.legend,
    style: {
      // IE 11: fieldset with legend does not render
      // a border radius. This maintains consistency
      // by always having a legend rendered
      width: s ? p : 0.01
    }
  }, /* @__PURE__ */ b.createElement("span", {
    dangerouslySetInnerHTML: {
      __html: "&#8203;"
    }
  })));
});
const q0 = et(K0, {
  name: "PrivateNotchedOutline"
})(U0);
var G0 = function(e) {
  var a = e.palette.type === "light" ? "rgba(0, 0, 0, 0.23)" : "rgba(255, 255, 255, 0.23)";
  return {
    /* Styles applied to the root element. */
    root: {
      position: "relative",
      borderRadius: e.shape.borderRadius,
      "&:hover $notchedOutline": {
        borderColor: e.palette.text.primary
      },
      // Reset on touch devices, it doesn't add specificity
      "@media (hover: none)": {
        "&:hover $notchedOutline": {
          borderColor: a
        }
      },
      "&$focused $notchedOutline": {
        borderColor: e.palette.primary.main,
        borderWidth: 2
      },
      "&$error $notchedOutline": {
        borderColor: e.palette.error.main
      },
      "&$disabled $notchedOutline": {
        borderColor: e.palette.action.disabled
      }
    },
    /* Styles applied to the root element if the color is secondary. */
    colorSecondary: {
      "&$focused $notchedOutline": {
        borderColor: e.palette.secondary.main
      }
    },
    /* Styles applied to the root element if the component is focused. */
    focused: {},
    /* Styles applied to the root element if `disabled={true}`. */
    disabled: {},
    /* Styles applied to the root element if `startAdornment` is provided. */
    adornedStart: {
      paddingLeft: 14
    },
    /* Styles applied to the root element if `endAdornment` is provided. */
    adornedEnd: {
      paddingRight: 14
    },
    /* Pseudo-class applied to the root element if `error={true}`. */
    error: {},
    /* Styles applied to the `input` element if `margin="dense"`. */
    marginDense: {},
    /* Styles applied to the root element if `multiline={true}`. */
    multiline: {
      padding: "18.5px 14px",
      "&$marginDense": {
        paddingTop: 10.5,
        paddingBottom: 10.5
      }
    },
    /* Styles applied to the `NotchedOutline` element. */
    notchedOutline: {
      borderColor: a
    },
    /* Styles applied to the `input` element. */
    input: {
      padding: "18.5px 14px",
      "&:-webkit-autofill": {
        WebkitBoxShadow: e.palette.type === "light" ? null : "0 0 0 100px #266798 inset",
        WebkitTextFillColor: e.palette.type === "light" ? null : "#fff",
        caretColor: e.palette.type === "light" ? null : "#fff",
        borderRadius: "inherit"
      }
    },
    /* Styles applied to the `input` element if `margin="dense"`. */
    inputMarginDense: {
      paddingTop: 10.5,
      paddingBottom: 10.5
    },
    /* Styles applied to the `input` element if `multiline={true}`. */
    inputMultiline: {
      padding: 0
    },
    /* Styles applied to the `input` element if `startAdornment` is provided. */
    inputAdornedStart: {
      paddingLeft: 0
    },
    /* Styles applied to the `input` element if `endAdornment` is provided. */
    inputAdornedEnd: {
      paddingRight: 0
    }
  };
}, yu = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.classes, n = e.fullWidth, o = n === void 0 ? !1 : n, i = e.inputComponent, s = i === void 0 ? "input" : i, l = e.label, u = e.labelWidth, c = u === void 0 ? 0 : u, d = e.multiline, p = d === void 0 ? !1 : d, h = e.notched, g = e.type, m = g === void 0 ? "text" : g, y = Ae(e, ["classes", "fullWidth", "inputComponent", "label", "labelWidth", "multiline", "notched", "type"]);
  return /* @__PURE__ */ b.createElement(ro, Y({
    renderSuffix: function(k) {
      return /* @__PURE__ */ b.createElement(q0, {
        className: r.notchedOutline,
        label: l,
        labelWidth: c,
        notched: typeof h < "u" ? h : !!(k.startAdornment || k.filled || k.focused)
      });
    },
    classes: Y({}, r, {
      root: he(r.root, r.underline),
      notchedOutline: null
    }),
    fullWidth: o,
    inputComponent: s,
    multiline: p,
    ref: a,
    type: m
  }, y));
});
yu.muiName = "Input";
const Y0 = et(G0, {
  name: "MuiOutlinedInput"
})(yu);
/**!
 * @fileOverview Kickass library to create and place poppers near their reference elements.
 * @version 1.16.1-lts
 * @license
 * Copyright (c) 2016 Federico Zivolo and contributors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */
var oa = typeof window < "u" && typeof document < "u" && typeof navigator < "u", X0 = (function() {
  for (var t = ["Edge", "Trident", "Firefox"], e = 0; e < t.length; e += 1)
    if (oa && navigator.userAgent.indexOf(t[e]) >= 0)
      return 1;
  return 0;
})();
function J0(t) {
  var e = !1;
  return function() {
    e || (e = !0, window.Promise.resolve().then(function() {
      e = !1, t();
    }));
  };
}
function Z0(t) {
  var e = !1;
  return function() {
    e || (e = !0, setTimeout(function() {
      e = !1, t();
    }, X0));
  };
}
var Q0 = oa && window.Promise, ev = Q0 ? J0 : Z0;
function xu(t) {
  var e = {};
  return t && e.toString.call(t) === "[object Function]";
}
function Ur(t, e) {
  if (t.nodeType !== 1)
    return [];
  var a = t.ownerDocument.defaultView, r = a.getComputedStyle(t, null);
  return e ? r[e] : r;
}
function us(t) {
  return t.nodeName === "HTML" ? t : t.parentNode || t.host;
}
function ia(t) {
  if (!t)
    return document.body;
  switch (t.nodeName) {
    case "HTML":
    case "BODY":
      return t.ownerDocument.body;
    case "#document":
      return t.body;
  }
  var e = Ur(t), a = e.overflow, r = e.overflowX, n = e.overflowY;
  return /(auto|scroll|overlay)/.test(a + n + r) ? t : ia(us(t));
}
function Cu(t) {
  return t && t.referenceNode ? t.referenceNode : t;
}
var jl = oa && !!(window.MSInputMethodContext && document.documentMode), Il = oa && /MSIE 10/.test(navigator.userAgent);
function Rn(t) {
  return t === 11 ? jl : t === 10 ? Il : jl || Il;
}
function bn(t) {
  if (!t)
    return document.documentElement;
  for (var e = Rn(10) ? document.body : null, a = t.offsetParent || null; a === e && t.nextElementSibling; )
    a = (t = t.nextElementSibling).offsetParent;
  var r = a && a.nodeName;
  return !r || r === "BODY" || r === "HTML" ? t ? t.ownerDocument.documentElement : document.documentElement : ["TH", "TD", "TABLE"].indexOf(a.nodeName) !== -1 && Ur(a, "position") === "static" ? bn(a) : a;
}
function tv(t) {
  var e = t.nodeName;
  return e === "BODY" ? !1 : e === "HTML" || bn(t.firstElementChild) === t;
}
function ji(t) {
  return t.parentNode !== null ? ji(t.parentNode) : t;
}
function Ha(t, e) {
  if (!t || !t.nodeType || !e || !e.nodeType)
    return document.documentElement;
  var a = t.compareDocumentPosition(e) & Node.DOCUMENT_POSITION_FOLLOWING, r = a ? t : e, n = a ? e : t, o = document.createRange();
  o.setStart(r, 0), o.setEnd(n, 0);
  var i = o.commonAncestorContainer;
  if (t !== i && e !== i || r.contains(n))
    return tv(i) ? i : bn(i);
  var s = ji(t);
  return s.host ? Ha(s.host, e) : Ha(t, ji(e).host);
}
function yn(t) {
  var e = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : "top", a = e === "top" ? "scrollTop" : "scrollLeft", r = t.nodeName;
  if (r === "BODY" || r === "HTML") {
    var n = t.ownerDocument.documentElement, o = t.ownerDocument.scrollingElement || n;
    return o[a];
  }
  return t[a];
}
function rv(t, e) {
  var a = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : !1, r = yn(e, "top"), n = yn(e, "left"), o = a ? -1 : 1;
  return t.top += r * o, t.bottom += r * o, t.left += n * o, t.right += n * o, t;
}
function Al(t, e) {
  var a = e === "x" ? "Left" : "Top", r = a === "Left" ? "Right" : "Bottom";
  return parseFloat(t["border" + a + "Width"]) + parseFloat(t["border" + r + "Width"]);
}
function Ml(t, e, a, r) {
  return Math.max(e["offset" + t], e["scroll" + t], a["client" + t], a["offset" + t], a["scroll" + t], Rn(10) ? parseInt(a["offset" + t]) + parseInt(r["margin" + (t === "Height" ? "Top" : "Left")]) + parseInt(r["margin" + (t === "Height" ? "Bottom" : "Right")]) : 0);
}
function Su(t) {
  var e = t.body, a = t.documentElement, r = Rn(10) && getComputedStyle(a);
  return {
    height: Ml("Height", e, a, r),
    width: Ml("Width", e, a, r)
  };
}
var nv = function(t, e) {
  if (!(t instanceof e))
    throw new TypeError("Cannot call a class as a function");
}, av = /* @__PURE__ */ (function() {
  function t(e, a) {
    for (var r = 0; r < a.length; r++) {
      var n = a[r];
      n.enumerable = n.enumerable || !1, n.configurable = !0, "value" in n && (n.writable = !0), Object.defineProperty(e, n.key, n);
    }
  }
  return function(e, a, r) {
    return a && t(e.prototype, a), r && t(e, r), e;
  };
})(), xn = function(t, e, a) {
  return e in t ? Object.defineProperty(t, e, {
    value: a,
    enumerable: !0,
    configurable: !0,
    writable: !0
  }) : t[e] = a, t;
}, Kt = Object.assign || function(t) {
  for (var e = 1; e < arguments.length; e++) {
    var a = arguments[e];
    for (var r in a)
      Object.prototype.hasOwnProperty.call(a, r) && (t[r] = a[r]);
  }
  return t;
};
function $r(t) {
  return Kt({}, t, {
    right: t.left + t.width,
    bottom: t.top + t.height
  });
}
function Ii(t) {
  var e = {};
  try {
    if (Rn(10)) {
      e = t.getBoundingClientRect();
      var a = yn(t, "top"), r = yn(t, "left");
      e.top += a, e.left += r, e.bottom += a, e.right += r;
    } else
      e = t.getBoundingClientRect();
  } catch {
  }
  var n = {
    left: e.left,
    top: e.top,
    width: e.right - e.left,
    height: e.bottom - e.top
  }, o = t.nodeName === "HTML" ? Su(t.ownerDocument) : {}, i = o.width || t.clientWidth || n.width, s = o.height || t.clientHeight || n.height, l = t.offsetWidth - i, u = t.offsetHeight - s;
  if (l || u) {
    var c = Ur(t);
    l -= Al(c, "x"), u -= Al(c, "y"), n.width -= l, n.height -= u;
  }
  return $r(n);
}
function ds(t, e) {
  var a = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : !1, r = Rn(10), n = e.nodeName === "HTML", o = Ii(t), i = Ii(e), s = ia(t), l = Ur(e), u = parseFloat(l.borderTopWidth), c = parseFloat(l.borderLeftWidth);
  a && n && (i.top = Math.max(i.top, 0), i.left = Math.max(i.left, 0));
  var d = $r({
    top: o.top - i.top - u,
    left: o.left - i.left - c,
    width: o.width,
    height: o.height
  });
  if (d.marginTop = 0, d.marginLeft = 0, !r && n) {
    var p = parseFloat(l.marginTop), h = parseFloat(l.marginLeft);
    d.top -= u - p, d.bottom -= u - p, d.left -= c - h, d.right -= c - h, d.marginTop = p, d.marginLeft = h;
  }
  return (r && !a ? e.contains(s) : e === s && s.nodeName !== "BODY") && (d = rv(d, e)), d;
}
function ov(t) {
  var e = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : !1, a = t.ownerDocument.documentElement, r = ds(t, a), n = Math.max(a.clientWidth, window.innerWidth || 0), o = Math.max(a.clientHeight, window.innerHeight || 0), i = e ? 0 : yn(a), s = e ? 0 : yn(a, "left"), l = {
    top: i - r.top + r.marginTop,
    left: s - r.left + r.marginLeft,
    width: n,
    height: o
  };
  return $r(l);
}
function wu(t) {
  var e = t.nodeName;
  if (e === "BODY" || e === "HTML")
    return !1;
  if (Ur(t, "position") === "fixed")
    return !0;
  var a = us(t);
  return a ? wu(a) : !1;
}
function Eu(t) {
  if (!t || !t.parentElement || Rn())
    return document.documentElement;
  for (var e = t.parentElement; e && Ur(e, "transform") === "none"; )
    e = e.parentElement;
  return e || document.documentElement;
}
function fs(t, e, a, r) {
  var n = arguments.length > 4 && arguments[4] !== void 0 ? arguments[4] : !1, o = { top: 0, left: 0 }, i = n ? Eu(t) : Ha(t, Cu(e));
  if (r === "viewport")
    o = ov(i, n);
  else {
    var s = void 0;
    r === "scrollParent" ? (s = ia(us(e)), s.nodeName === "BODY" && (s = t.ownerDocument.documentElement)) : r === "window" ? s = t.ownerDocument.documentElement : s = r;
    var l = ds(s, i, n);
    if (s.nodeName === "HTML" && !wu(i)) {
      var u = Su(t.ownerDocument), c = u.height, d = u.width;
      o.top += l.top - l.marginTop, o.bottom = c + l.top, o.left += l.left - l.marginLeft, o.right = d + l.left;
    } else
      o = l;
  }
  a = a || 0;
  var p = typeof a == "number";
  return o.left += p ? a : a.left || 0, o.top += p ? a : a.top || 0, o.right -= p ? a : a.right || 0, o.bottom -= p ? a : a.bottom || 0, o;
}
function iv(t) {
  var e = t.width, a = t.height;
  return e * a;
}
function ku(t, e, a, r, n) {
  var o = arguments.length > 5 && arguments[5] !== void 0 ? arguments[5] : 0;
  if (t.indexOf("auto") === -1)
    return t;
  var i = fs(a, r, o, n), s = {
    top: {
      width: i.width,
      height: e.top - i.top
    },
    right: {
      width: i.right - e.right,
      height: i.height
    },
    bottom: {
      width: i.width,
      height: i.bottom - e.bottom
    },
    left: {
      width: e.left - i.left,
      height: i.height
    }
  }, l = Object.keys(s).map(function(p) {
    return Kt({
      key: p
    }, s[p], {
      area: iv(s[p])
    });
  }).sort(function(p, h) {
    return h.area - p.area;
  }), u = l.filter(function(p) {
    var h = p.width, g = p.height;
    return h >= a.clientWidth && g >= a.clientHeight;
  }), c = u.length > 0 ? u[0].key : l[0].key, d = t.split("-")[1];
  return c + (d ? "-" + d : "");
}
function $u(t, e, a) {
  var r = arguments.length > 3 && arguments[3] !== void 0 ? arguments[3] : null, n = r ? Eu(e) : Ha(e, Cu(a));
  return ds(a, n, r);
}
function Ru(t) {
  var e = t.ownerDocument.defaultView, a = e.getComputedStyle(t), r = parseFloat(a.marginTop || 0) + parseFloat(a.marginBottom || 0), n = parseFloat(a.marginLeft || 0) + parseFloat(a.marginRight || 0), o = {
    width: t.offsetWidth + n,
    height: t.offsetHeight + r
  };
  return o;
}
function Ka(t) {
  var e = { left: "right", right: "left", bottom: "top", top: "bottom" };
  return t.replace(/left|right|bottom|top/g, function(a) {
    return e[a];
  });
}
function Tu(t, e, a) {
  a = a.split("-")[0];
  var r = Ru(t), n = {
    width: r.width,
    height: r.height
  }, o = ["right", "left"].indexOf(a) !== -1, i = o ? "top" : "left", s = o ? "left" : "top", l = o ? "height" : "width", u = o ? "width" : "height";
  return n[i] = e[i] + e[l] / 2 - r[l] / 2, a === s ? n[s] = e[s] - r[u] : n[s] = e[Ka(s)], n;
}
function sa(t, e) {
  return Array.prototype.find ? t.find(e) : t.filter(e)[0];
}
function sv(t, e, a) {
  if (Array.prototype.findIndex)
    return t.findIndex(function(n) {
      return n[e] === a;
    });
  var r = sa(t, function(n) {
    return n[e] === a;
  });
  return t.indexOf(r);
}
function Ou(t, e, a) {
  var r = a === void 0 ? t : t.slice(0, sv(t, "name", a));
  return r.forEach(function(n) {
    n.function && console.warn("`modifier.function` is deprecated, use `modifier.fn`!");
    var o = n.function || n.fn;
    n.enabled && xu(o) && (e.offsets.popper = $r(e.offsets.popper), e.offsets.reference = $r(e.offsets.reference), e = o(e, n));
  }), e;
}
function lv() {
  if (!this.state.isDestroyed) {
    var t = {
      instance: this,
      styles: {},
      arrowStyles: {},
      attributes: {},
      flipped: !1,
      offsets: {}
    };
    t.offsets.reference = $u(this.state, this.popper, this.reference, this.options.positionFixed), t.placement = ku(this.options.placement, t.offsets.reference, this.popper, this.reference, this.options.modifiers.flip.boundariesElement, this.options.modifiers.flip.padding), t.originalPlacement = t.placement, t.positionFixed = this.options.positionFixed, t.offsets.popper = Tu(this.popper, t.offsets.reference, t.placement), t.offsets.popper.position = this.options.positionFixed ? "fixed" : "absolute", t = Ou(this.modifiers, t), this.state.isCreated ? this.options.onUpdate(t) : (this.state.isCreated = !0, this.options.onCreate(t));
  }
}
function _u(t, e) {
  return t.some(function(a) {
    var r = a.name, n = a.enabled;
    return n && r === e;
  });
}
function ps(t) {
  for (var e = [!1, "ms", "Webkit", "Moz", "O"], a = t.charAt(0).toUpperCase() + t.slice(1), r = 0; r < e.length; r++) {
    var n = e[r], o = n ? "" + n + a : t;
    if (typeof document.body.style[o] < "u")
      return o;
  }
  return null;
}
function cv() {
  return this.state.isDestroyed = !0, _u(this.modifiers, "applyStyle") && (this.popper.removeAttribute("x-placement"), this.popper.style.position = "", this.popper.style.top = "", this.popper.style.left = "", this.popper.style.right = "", this.popper.style.bottom = "", this.popper.style.willChange = "", this.popper.style[ps("transform")] = ""), this.disableEventListeners(), this.options.removeOnDestroy && this.popper.parentNode.removeChild(this.popper), this;
}
function Pu(t) {
  var e = t.ownerDocument;
  return e ? e.defaultView : window;
}
function ju(t, e, a, r) {
  var n = t.nodeName === "BODY", o = n ? t.ownerDocument.defaultView : t;
  o.addEventListener(e, a, { passive: !0 }), n || ju(ia(o.parentNode), e, a, r), r.push(o);
}
function uv(t, e, a, r) {
  a.updateBound = r, Pu(t).addEventListener("resize", a.updateBound, { passive: !0 });
  var n = ia(t);
  return ju(n, "scroll", a.updateBound, a.scrollParents), a.scrollElement = n, a.eventsEnabled = !0, a;
}
function dv() {
  this.state.eventsEnabled || (this.state = uv(this.reference, this.options, this.state, this.scheduleUpdate));
}
function fv(t, e) {
  return Pu(t).removeEventListener("resize", e.updateBound), e.scrollParents.forEach(function(a) {
    a.removeEventListener("scroll", e.updateBound);
  }), e.updateBound = null, e.scrollParents = [], e.scrollElement = null, e.eventsEnabled = !1, e;
}
function pv() {
  this.state.eventsEnabled && (cancelAnimationFrame(this.scheduleUpdate), this.state = fv(this.reference, this.state));
}
function hs(t) {
  return t !== "" && !isNaN(parseFloat(t)) && isFinite(t);
}
function Ai(t, e) {
  Object.keys(e).forEach(function(a) {
    var r = "";
    ["width", "height", "top", "right", "bottom", "left"].indexOf(a) !== -1 && hs(e[a]) && (r = "px"), t.style[a] = e[a] + r;
  });
}
function hv(t, e) {
  Object.keys(e).forEach(function(a) {
    var r = e[a];
    r !== !1 ? t.setAttribute(a, e[a]) : t.removeAttribute(a);
  });
}
function mv(t) {
  return Ai(t.instance.popper, t.styles), hv(t.instance.popper, t.attributes), t.arrowElement && Object.keys(t.arrowStyles).length && Ai(t.arrowElement, t.arrowStyles), t;
}
function gv(t, e, a, r, n) {
  var o = $u(n, e, t, a.positionFixed), i = ku(a.placement, o, e, t, a.modifiers.flip.boundariesElement, a.modifiers.flip.padding);
  return e.setAttribute("x-placement", i), Ai(e, { position: a.positionFixed ? "fixed" : "absolute" }), a;
}
function vv(t, e) {
  var a = t.offsets, r = a.popper, n = a.reference, o = Math.round, i = Math.floor, s = function(w) {
    return w;
  }, l = o(n.width), u = o(r.width), c = ["left", "right"].indexOf(t.placement) !== -1, d = t.placement.indexOf("-") !== -1, p = l % 2 === u % 2, h = l % 2 === 1 && u % 2 === 1, g = e ? c || d || p ? o : i : s, m = e ? o : s;
  return {
    left: g(h && !d && e ? r.left - 1 : r.left),
    top: m(r.top),
    bottom: m(r.bottom),
    right: g(r.right)
  };
}
var bv = oa && /Firefox/i.test(navigator.userAgent);
function yv(t, e) {
  var a = e.x, r = e.y, n = t.offsets.popper, o = sa(t.instance.modifiers, function(j) {
    return j.name === "applyStyle";
  }).gpuAcceleration;
  o !== void 0 && console.warn("WARNING: `gpuAcceleration` option moved to `computeStyle` modifier and will not be supported in future versions of Popper.js!");
  var i = o !== void 0 ? o : e.gpuAcceleration, s = bn(t.instance.popper), l = Ii(s), u = {
    position: n.position
  }, c = vv(t, window.devicePixelRatio < 2 || !bv), d = a === "bottom" ? "top" : "bottom", p = r === "right" ? "left" : "right", h = ps("transform"), g = void 0, m = void 0;
  if (d === "bottom" ? s.nodeName === "HTML" ? m = -s.clientHeight + c.bottom : m = -l.height + c.bottom : m = c.top, p === "right" ? s.nodeName === "HTML" ? g = -s.clientWidth + c.right : g = -l.width + c.right : g = c.left, i && h)
    u[h] = "translate3d(" + g + "px, " + m + "px, 0)", u[d] = 0, u[p] = 0, u.willChange = "transform";
  else {
    var y = d === "bottom" ? -1 : 1, w = p === "right" ? -1 : 1;
    u[d] = m * y, u[p] = g * w, u.willChange = d + ", " + p;
  }
  var k = {
    "x-placement": t.placement
  };
  return t.attributes = Kt({}, k, t.attributes), t.styles = Kt({}, u, t.styles), t.arrowStyles = Kt({}, t.offsets.arrow, t.arrowStyles), t;
}
function Iu(t, e, a) {
  var r = sa(t, function(s) {
    var l = s.name;
    return l === e;
  }), n = !!r && t.some(function(s) {
    return s.name === a && s.enabled && s.order < r.order;
  });
  if (!n) {
    var o = "`" + e + "`", i = "`" + a + "`";
    console.warn(i + " modifier is required by " + o + " modifier in order to work, be sure to include it before " + o + "!");
  }
  return n;
}
function xv(t, e) {
  var a;
  if (!Iu(t.instance.modifiers, "arrow", "keepTogether"))
    return t;
  var r = e.element;
  if (typeof r == "string") {
    if (r = t.instance.popper.querySelector(r), !r)
      return t;
  } else if (!t.instance.popper.contains(r))
    return console.warn("WARNING: `arrow.element` must be child of its popper element!"), t;
  var n = t.placement.split("-")[0], o = t.offsets, i = o.popper, s = o.reference, l = ["left", "right"].indexOf(n) !== -1, u = l ? "height" : "width", c = l ? "Top" : "Left", d = c.toLowerCase(), p = l ? "left" : "top", h = l ? "bottom" : "right", g = Ru(r)[u];
  s[h] - g < i[d] && (t.offsets.popper[d] -= i[d] - (s[h] - g)), s[d] + g > i[h] && (t.offsets.popper[d] += s[d] + g - i[h]), t.offsets.popper = $r(t.offsets.popper);
  var m = s[d] + s[u] / 2 - g / 2, y = Ur(t.instance.popper), w = parseFloat(y["margin" + c]), k = parseFloat(y["border" + c + "Width"]), j = m - t.offsets.popper[d] - w - k;
  return j = Math.max(Math.min(i[u] - g, j), 0), t.arrowElement = r, t.offsets.arrow = (a = {}, xn(a, d, Math.round(j)), xn(a, p, ""), a), t;
}
function Cv(t) {
  return t === "end" ? "start" : t === "start" ? "end" : t;
}
var Au = ["auto-start", "auto", "auto-end", "top-start", "top", "top-end", "right-start", "right", "right-end", "bottom-end", "bottom", "bottom-start", "left-end", "left", "left-start"], ri = Au.slice(3);
function Nl(t) {
  var e = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : !1, a = ri.indexOf(t), r = ri.slice(a + 1).concat(ri.slice(0, a));
  return e ? r.reverse() : r;
}
var ni = {
  FLIP: "flip",
  CLOCKWISE: "clockwise",
  COUNTERCLOCKWISE: "counterclockwise"
};
function Sv(t, e) {
  if (_u(t.instance.modifiers, "inner") || t.flipped && t.placement === t.originalPlacement)
    return t;
  var a = fs(t.instance.popper, t.instance.reference, e.padding, e.boundariesElement, t.positionFixed), r = t.placement.split("-")[0], n = Ka(r), o = t.placement.split("-")[1] || "", i = [];
  switch (e.behavior) {
    case ni.FLIP:
      i = [r, n];
      break;
    case ni.CLOCKWISE:
      i = Nl(r);
      break;
    case ni.COUNTERCLOCKWISE:
      i = Nl(r, !0);
      break;
    default:
      i = e.behavior;
  }
  return i.forEach(function(s, l) {
    if (r !== s || i.length === l + 1)
      return t;
    r = t.placement.split("-")[0], n = Ka(r);
    var u = t.offsets.popper, c = t.offsets.reference, d = Math.floor, p = r === "left" && d(u.right) > d(c.left) || r === "right" && d(u.left) < d(c.right) || r === "top" && d(u.bottom) > d(c.top) || r === "bottom" && d(u.top) < d(c.bottom), h = d(u.left) < d(a.left), g = d(u.right) > d(a.right), m = d(u.top) < d(a.top), y = d(u.bottom) > d(a.bottom), w = r === "left" && h || r === "right" && g || r === "top" && m || r === "bottom" && y, k = ["top", "bottom"].indexOf(r) !== -1, j = !!e.flipVariations && (k && o === "start" && h || k && o === "end" && g || !k && o === "start" && m || !k && o === "end" && y), f = !!e.flipVariationsByContent && (k && o === "start" && g || k && o === "end" && h || !k && o === "start" && y || !k && o === "end" && m), x = j || f;
    (p || w || x) && (t.flipped = !0, (p || w) && (r = i[l + 1]), x && (o = Cv(o)), t.placement = r + (o ? "-" + o : ""), t.offsets.popper = Kt({}, t.offsets.popper, Tu(t.instance.popper, t.offsets.reference, t.placement)), t = Ou(t.instance.modifiers, t, "flip"));
  }), t;
}
function wv(t) {
  var e = t.offsets, a = e.popper, r = e.reference, n = t.placement.split("-")[0], o = Math.floor, i = ["top", "bottom"].indexOf(n) !== -1, s = i ? "right" : "bottom", l = i ? "left" : "top", u = i ? "width" : "height";
  return a[s] < o(r[l]) && (t.offsets.popper[l] = o(r[l]) - a[u]), a[l] > o(r[s]) && (t.offsets.popper[l] = o(r[s])), t;
}
function Ev(t, e, a, r) {
  var n = t.match(/((?:\-|\+)?\d*\.?\d*)(.*)/), o = +n[1], i = n[2];
  if (!o)
    return t;
  if (i.indexOf("%") === 0) {
    var s = void 0;
    switch (i) {
      case "%p":
        s = a;
        break;
      case "%":
      case "%r":
      default:
        s = r;
    }
    var l = $r(s);
    return l[e] / 100 * o;
  } else if (i === "vh" || i === "vw") {
    var u = void 0;
    return i === "vh" ? u = Math.max(document.documentElement.clientHeight, window.innerHeight || 0) : u = Math.max(document.documentElement.clientWidth, window.innerWidth || 0), u / 100 * o;
  } else
    return o;
}
function kv(t, e, a, r) {
  var n = [0, 0], o = ["right", "left"].indexOf(r) !== -1, i = t.split(/(\+|\-)/).map(function(c) {
    return c.trim();
  }), s = i.indexOf(sa(i, function(c) {
    return c.search(/,|\s/) !== -1;
  }));
  i[s] && i[s].indexOf(",") === -1 && console.warn("Offsets separated by white space(s) are deprecated, use a comma (,) instead.");
  var l = /\s*,\s*|\s+/, u = s !== -1 ? [i.slice(0, s).concat([i[s].split(l)[0]]), [i[s].split(l)[1]].concat(i.slice(s + 1))] : [i];
  return u = u.map(function(c, d) {
    var p = (d === 1 ? !o : o) ? "height" : "width", h = !1;
    return c.reduce(function(g, m) {
      return g[g.length - 1] === "" && ["+", "-"].indexOf(m) !== -1 ? (g[g.length - 1] = m, h = !0, g) : h ? (g[g.length - 1] += m, h = !1, g) : g.concat(m);
    }, []).map(function(g) {
      return Ev(g, p, e, a);
    });
  }), u.forEach(function(c, d) {
    c.forEach(function(p, h) {
      hs(p) && (n[d] += p * (c[h - 1] === "-" ? -1 : 1));
    });
  }), n;
}
function $v(t, e) {
  var a = e.offset, r = t.placement, n = t.offsets, o = n.popper, i = n.reference, s = r.split("-")[0], l = void 0;
  return hs(+a) ? l = [+a, 0] : l = kv(a, o, i, s), s === "left" ? (o.top += l[0], o.left -= l[1]) : s === "right" ? (o.top += l[0], o.left += l[1]) : s === "top" ? (o.left += l[0], o.top -= l[1]) : s === "bottom" && (o.left += l[0], o.top += l[1]), t.popper = o, t;
}
function Rv(t, e) {
  var a = e.boundariesElement || bn(t.instance.popper);
  t.instance.reference === a && (a = bn(a));
  var r = ps("transform"), n = t.instance.popper.style, o = n.top, i = n.left, s = n[r];
  n.top = "", n.left = "", n[r] = "";
  var l = fs(t.instance.popper, t.instance.reference, e.padding, a, t.positionFixed);
  n.top = o, n.left = i, n[r] = s, e.boundaries = l;
  var u = e.priority, c = t.offsets.popper, d = {
    primary: function(h) {
      var g = c[h];
      return c[h] < l[h] && !e.escapeWithReference && (g = Math.max(c[h], l[h])), xn({}, h, g);
    },
    secondary: function(h) {
      var g = h === "right" ? "left" : "top", m = c[g];
      return c[h] > l[h] && !e.escapeWithReference && (m = Math.min(c[g], l[h] - (h === "right" ? c.width : c.height))), xn({}, g, m);
    }
  };
  return u.forEach(function(p) {
    var h = ["left", "top"].indexOf(p) !== -1 ? "primary" : "secondary";
    c = Kt({}, c, d[h](p));
  }), t.offsets.popper = c, t;
}
function Tv(t) {
  var e = t.placement, a = e.split("-")[0], r = e.split("-")[1];
  if (r) {
    var n = t.offsets, o = n.reference, i = n.popper, s = ["bottom", "top"].indexOf(a) !== -1, l = s ? "left" : "top", u = s ? "width" : "height", c = {
      start: xn({}, l, o[l]),
      end: xn({}, l, o[l] + o[u] - i[u])
    };
    t.offsets.popper = Kt({}, i, c[r]);
  }
  return t;
}
function Ov(t) {
  if (!Iu(t.instance.modifiers, "hide", "preventOverflow"))
    return t;
  var e = t.offsets.reference, a = sa(t.instance.modifiers, function(r) {
    return r.name === "preventOverflow";
  }).boundaries;
  if (e.bottom < a.top || e.left > a.right || e.top > a.bottom || e.right < a.left) {
    if (t.hide === !0)
      return t;
    t.hide = !0, t.attributes["x-out-of-boundaries"] = "";
  } else {
    if (t.hide === !1)
      return t;
    t.hide = !1, t.attributes["x-out-of-boundaries"] = !1;
  }
  return t;
}
function _v(t) {
  var e = t.placement, a = e.split("-")[0], r = t.offsets, n = r.popper, o = r.reference, i = ["left", "right"].indexOf(a) !== -1, s = ["top", "left"].indexOf(a) === -1;
  return n[i ? "left" : "top"] = o[a] - (s ? n[i ? "width" : "height"] : 0), t.placement = Ka(e), t.offsets.popper = $r(n), t;
}
var Pv = {
  /**
   * Modifier used to shift the popper on the start or end of its reference
   * element.<br />
   * It will read the variation of the `placement` property.<br />
   * It can be one either `-end` or `-start`.
   * @memberof modifiers
   * @inner
   */
  shift: {
    /** @prop {number} order=100 - Index used to define the order of execution */
    order: 100,
    /** @prop {Boolean} enabled=true - Whether the modifier is enabled or not */
    enabled: !0,
    /** @prop {ModifierFn} */
    fn: Tv
  },
  /**
   * The `offset` modifier can shift your popper on both its axis.
   *
   * It accepts the following units:
   * - `px` or unit-less, interpreted as pixels
   * - `%` or `%r`, percentage relative to the length of the reference element
   * - `%p`, percentage relative to the length of the popper element
   * - `vw`, CSS viewport width unit
   * - `vh`, CSS viewport height unit
   *
   * For length is intended the main axis relative to the placement of the popper.<br />
   * This means that if the placement is `top` or `bottom`, the length will be the
   * `width`. In case of `left` or `right`, it will be the `height`.
   *
   * You can provide a single value (as `Number` or `String`), or a pair of values
   * as `String` divided by a comma or one (or more) white spaces.<br />
   * The latter is a deprecated method because it leads to confusion and will be
   * removed in v2.<br />
   * Additionally, it accepts additions and subtractions between different units.
   * Note that multiplications and divisions aren't supported.
   *
   * Valid examples are:
   * ```
   * 10
   * '10%'
   * '10, 10'
   * '10%, 10'
   * '10 + 10%'
   * '10 - 5vh + 3%'
   * '-10px + 5vh, 5px - 6%'
   * ```
   * > **NB**: If you desire to apply offsets to your poppers in a way that may make them overlap
   * > with their reference element, unfortunately, you will have to disable the `flip` modifier.
   * > You can read more on this at this [issue](https://github.com/FezVrasta/popper.js/issues/373).
   *
   * @memberof modifiers
   * @inner
   */
  offset: {
    /** @prop {number} order=200 - Index used to define the order of execution */
    order: 200,
    /** @prop {Boolean} enabled=true - Whether the modifier is enabled or not */
    enabled: !0,
    /** @prop {ModifierFn} */
    fn: $v,
    /** @prop {Number|String} offset=0
     * The offset value as described in the modifier description
     */
    offset: 0
  },
  /**
   * Modifier used to prevent the popper from being positioned outside the boundary.
   *
   * A scenario exists where the reference itself is not within the boundaries.<br />
   * We can say it has "escaped the boundaries" — or just "escaped".<br />
   * In this case we need to decide whether the popper should either:
   *
   * - detach from the reference and remain "trapped" in the boundaries, or
   * - if it should ignore the boundary and "escape with its reference"
   *
   * When `escapeWithReference` is set to`true` and reference is completely
   * outside its boundaries, the popper will overflow (or completely leave)
   * the boundaries in order to remain attached to the edge of the reference.
   *
   * @memberof modifiers
   * @inner
   */
  preventOverflow: {
    /** @prop {number} order=300 - Index used to define the order of execution */
    order: 300,
    /** @prop {Boolean} enabled=true - Whether the modifier is enabled or not */
    enabled: !0,
    /** @prop {ModifierFn} */
    fn: Rv,
    /**
     * @prop {Array} [priority=['left','right','top','bottom']]
     * Popper will try to prevent overflow following these priorities by default,
     * then, it could overflow on the left and on top of the `boundariesElement`
     */
    priority: ["left", "right", "top", "bottom"],
    /**
     * @prop {number} padding=5
     * Amount of pixel used to define a minimum distance between the boundaries
     * and the popper. This makes sure the popper always has a little padding
     * between the edges of its container
     */
    padding: 5,
    /**
     * @prop {String|HTMLElement} boundariesElement='scrollParent'
     * Boundaries used by the modifier. Can be `scrollParent`, `window`,
     * `viewport` or any DOM element.
     */
    boundariesElement: "scrollParent"
  },
  /**
   * Modifier used to make sure the reference and its popper stay near each other
   * without leaving any gap between the two. Especially useful when the arrow is
   * enabled and you want to ensure that it points to its reference element.
   * It cares only about the first axis. You can still have poppers with margin
   * between the popper and its reference element.
   * @memberof modifiers
   * @inner
   */
  keepTogether: {
    /** @prop {number} order=400 - Index used to define the order of execution */
    order: 400,
    /** @prop {Boolean} enabled=true - Whether the modifier is enabled or not */
    enabled: !0,
    /** @prop {ModifierFn} */
    fn: wv
  },
  /**
   * This modifier is used to move the `arrowElement` of the popper to make
   * sure it is positioned between the reference element and its popper element.
   * It will read the outer size of the `arrowElement` node to detect how many
   * pixels of conjunction are needed.
   *
   * It has no effect if no `arrowElement` is provided.
   * @memberof modifiers
   * @inner
   */
  arrow: {
    /** @prop {number} order=500 - Index used to define the order of execution */
    order: 500,
    /** @prop {Boolean} enabled=true - Whether the modifier is enabled or not */
    enabled: !0,
    /** @prop {ModifierFn} */
    fn: xv,
    /** @prop {String|HTMLElement} element='[x-arrow]' - Selector or node used as arrow */
    element: "[x-arrow]"
  },
  /**
   * Modifier used to flip the popper's placement when it starts to overlap its
   * reference element.
   *
   * Requires the `preventOverflow` modifier before it in order to work.
   *
   * **NOTE:** this modifier will interrupt the current update cycle and will
   * restart it if it detects the need to flip the placement.
   * @memberof modifiers
   * @inner
   */
  flip: {
    /** @prop {number} order=600 - Index used to define the order of execution */
    order: 600,
    /** @prop {Boolean} enabled=true - Whether the modifier is enabled or not */
    enabled: !0,
    /** @prop {ModifierFn} */
    fn: Sv,
    /**
     * @prop {String|Array} behavior='flip'
     * The behavior used to change the popper's placement. It can be one of
     * `flip`, `clockwise`, `counterclockwise` or an array with a list of valid
     * placements (with optional variations)
     */
    behavior: "flip",
    /**
     * @prop {number} padding=5
     * The popper will flip if it hits the edges of the `boundariesElement`
     */
    padding: 5,
    /**
     * @prop {String|HTMLElement} boundariesElement='viewport'
     * The element which will define the boundaries of the popper position.
     * The popper will never be placed outside of the defined boundaries
     * (except if `keepTogether` is enabled)
     */
    boundariesElement: "viewport",
    /**
     * @prop {Boolean} flipVariations=false
     * The popper will switch placement variation between `-start` and `-end` when
     * the reference element overlaps its boundaries.
     *
     * The original placement should have a set variation.
     */
    flipVariations: !1,
    /**
     * @prop {Boolean} flipVariationsByContent=false
     * The popper will switch placement variation between `-start` and `-end` when
     * the popper element overlaps its reference boundaries.
     *
     * The original placement should have a set variation.
     */
    flipVariationsByContent: !1
  },
  /**
   * Modifier used to make the popper flow toward the inner of the reference element.
   * By default, when this modifier is disabled, the popper will be placed outside
   * the reference element.
   * @memberof modifiers
   * @inner
   */
  inner: {
    /** @prop {number} order=700 - Index used to define the order of execution */
    order: 700,
    /** @prop {Boolean} enabled=false - Whether the modifier is enabled or not */
    enabled: !1,
    /** @prop {ModifierFn} */
    fn: _v
  },
  /**
   * Modifier used to hide the popper when its reference element is outside of the
   * popper boundaries. It will set a `x-out-of-boundaries` attribute which can
   * be used to hide with a CSS selector the popper when its reference is
   * out of boundaries.
   *
   * Requires the `preventOverflow` modifier before it in order to work.
   * @memberof modifiers
   * @inner
   */
  hide: {
    /** @prop {number} order=800 - Index used to define the order of execution */
    order: 800,
    /** @prop {Boolean} enabled=true - Whether the modifier is enabled or not */
    enabled: !0,
    /** @prop {ModifierFn} */
    fn: Ov
  },
  /**
   * Computes the style that will be applied to the popper element to gets
   * properly positioned.
   *
   * Note that this modifier will not touch the DOM, it just prepares the styles
   * so that `applyStyle` modifier can apply it. This separation is useful
   * in case you need to replace `applyStyle` with a custom implementation.
   *
   * This modifier has `850` as `order` value to maintain backward compatibility
   * with previous versions of Popper.js. Expect the modifiers ordering method
   * to change in future major versions of the library.
   *
   * @memberof modifiers
   * @inner
   */
  computeStyle: {
    /** @prop {number} order=850 - Index used to define the order of execution */
    order: 850,
    /** @prop {Boolean} enabled=true - Whether the modifier is enabled or not */
    enabled: !0,
    /** @prop {ModifierFn} */
    fn: yv,
    /**
     * @prop {Boolean} gpuAcceleration=true
     * If true, it uses the CSS 3D transformation to position the popper.
     * Otherwise, it will use the `top` and `left` properties
     */
    gpuAcceleration: !0,
    /**
     * @prop {string} [x='bottom']
     * Where to anchor the X axis (`bottom` or `top`). AKA X offset origin.
     * Change this if your popper should grow in a direction different from `bottom`
     */
    x: "bottom",
    /**
     * @prop {string} [x='left']
     * Where to anchor the Y axis (`left` or `right`). AKA Y offset origin.
     * Change this if your popper should grow in a direction different from `right`
     */
    y: "right"
  },
  /**
   * Applies the computed styles to the popper element.
   *
   * All the DOM manipulations are limited to this modifier. This is useful in case
   * you want to integrate Popper.js inside a framework or view library and you
   * want to delegate all the DOM manipulations to it.
   *
   * Note that if you disable this modifier, you must make sure the popper element
   * has its position set to `absolute` before Popper.js can do its work!
   *
   * Just disable this modifier and define your own to achieve the desired effect.
   *
   * @memberof modifiers
   * @inner
   */
  applyStyle: {
    /** @prop {number} order=900 - Index used to define the order of execution */
    order: 900,
    /** @prop {Boolean} enabled=true - Whether the modifier is enabled or not */
    enabled: !0,
    /** @prop {ModifierFn} */
    fn: mv,
    /** @prop {Function} */
    onLoad: gv,
    /**
     * @deprecated since version 1.10.0, the property moved to `computeStyle` modifier
     * @prop {Boolean} gpuAcceleration=true
     * If true, it uses the CSS 3D transformation to position the popper.
     * Otherwise, it will use the `top` and `left` properties
     */
    gpuAcceleration: void 0
  }
}, jv = {
  /**
   * Popper's placement.
   * @prop {Popper.placements} placement='bottom'
   */
  placement: "bottom",
  /**
   * Set this to true if you want popper to position it self in 'fixed' mode
   * @prop {Boolean} positionFixed=false
   */
  positionFixed: !1,
  /**
   * Whether events (resize, scroll) are initially enabled.
   * @prop {Boolean} eventsEnabled=true
   */
  eventsEnabled: !0,
  /**
   * Set to true if you want to automatically remove the popper when
   * you call the `destroy` method.
   * @prop {Boolean} removeOnDestroy=false
   */
  removeOnDestroy: !1,
  /**
   * Callback called when the popper is created.<br />
   * By default, it is set to no-op.<br />
   * Access Popper.js instance with `data.instance`.
   * @prop {onCreate}
   */
  onCreate: function() {
  },
  /**
   * Callback called when the popper is updated. This callback is not called
   * on the initialization/creation of the popper, but only on subsequent
   * updates.<br />
   * By default, it is set to no-op.<br />
   * Access Popper.js instance with `data.instance`.
   * @prop {onUpdate}
   */
  onUpdate: function() {
  },
  /**
   * List of modifiers used to modify the offsets before they are applied to the popper.
   * They provide most of the functionalities of Popper.js.
   * @prop {modifiers}
   */
  modifiers: Pv
}, no = (function() {
  function t(e, a) {
    var r = this, n = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : {};
    nv(this, t), this.scheduleUpdate = function() {
      return requestAnimationFrame(r.update);
    }, this.update = ev(this.update.bind(this)), this.options = Kt({}, t.Defaults, n), this.state = {
      isDestroyed: !1,
      isCreated: !1,
      scrollParents: []
    }, this.reference = e && e.jquery ? e[0] : e, this.popper = a && a.jquery ? a[0] : a, this.options.modifiers = {}, Object.keys(Kt({}, t.Defaults.modifiers, n.modifiers)).forEach(function(i) {
      r.options.modifiers[i] = Kt({}, t.Defaults.modifiers[i] || {}, n.modifiers ? n.modifiers[i] : {});
    }), this.modifiers = Object.keys(this.options.modifiers).map(function(i) {
      return Kt({
        name: i
      }, r.options.modifiers[i]);
    }).sort(function(i, s) {
      return i.order - s.order;
    }), this.modifiers.forEach(function(i) {
      i.enabled && xu(i.onLoad) && i.onLoad(r.reference, r.popper, r.options, i, r.state);
    }), this.update();
    var o = this.options.eventsEnabled;
    o && this.enableEventListeners(), this.state.eventsEnabled = o;
  }
  return av(t, [{
    key: "update",
    value: function() {
      return lv.call(this);
    }
  }, {
    key: "destroy",
    value: function() {
      return cv.call(this);
    }
  }, {
    key: "enableEventListeners",
    value: function() {
      return dv.call(this);
    }
  }, {
    key: "disableEventListeners",
    value: function() {
      return pv.call(this);
    }
    /**
     * Schedules an update. It will run on the next UI update available.
     * @method scheduleUpdate
     * @memberof Popper
     */
    /**
     * Collection of utilities useful when writing custom modifiers.
     * Starting from version 1.7, this method is available only if you
     * include `popper-utils.js` before `popper.js`.
     *
     * **DEPRECATION**: This way to access PopperUtils is deprecated
     * and will be removed in v2! Use the PopperUtils module directly instead.
     * Due to the high instability of the methods contained in Utils, we can't
     * guarantee them to follow semver. Use them at your own risk!
     * @static
     * @private
     * @type {Object}
     * @deprecated since version 1.8
     * @member Utils
     * @memberof Popper
     */
  }]), t;
})();
no.Utils = (typeof window < "u" ? window : global).PopperUtils;
no.placements = Au;
no.Defaults = jv;
function Iv(t, e) {
  var a = e && e.direction || "ltr";
  if (a === "ltr")
    return t;
  switch (t) {
    case "bottom-end":
      return "bottom-start";
    case "bottom-start":
      return "bottom-end";
    case "top-end":
      return "top-start";
    case "top-start":
      return "top-end";
    default:
      return t;
  }
}
function Ll(t) {
  return typeof t == "function" ? t() : t;
}
var Av = typeof window < "u" ? b.useLayoutEffect : b.useEffect, Mv = {}, Nv = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.anchorEl, n = e.children, o = e.container, i = e.disablePortal, s = i === void 0 ? !1 : i, l = e.keepMounted, u = l === void 0 ? !1 : l, c = e.modifiers, d = e.open, p = e.placement, h = p === void 0 ? "bottom" : p, g = e.popperOptions, m = g === void 0 ? Mv : g, y = e.popperRef, w = e.style, k = e.transition, j = k === void 0 ? !1 : k, f = Ae(e, ["anchorEl", "children", "container", "disablePortal", "keepMounted", "modifiers", "open", "placement", "popperOptions", "popperRef", "style", "transition"]), x = b.useRef(null), E = St(x, a), L = b.useRef(null), W = St(L, y), O = b.useRef(W);
  Av(function() {
    O.current = W;
  }, [W]), b.useImperativeHandle(y, function() {
    return L.current;
  }, []);
  var P = b.useState(!0), A = P[0], v = P[1], C = na(), M = Iv(h, C), D = b.useState(M), $ = D[0], H = D[1];
  b.useEffect(function() {
    L.current && L.current.update();
  });
  var F = b.useCallback(function() {
    if (!(!x.current || !r || !d)) {
      L.current && (L.current.destroy(), O.current(null));
      var de = function(ye) {
        H(ye.placement);
      };
      Ll(r);
      var Se = new no(Ll(r), x.current, Y({
        placement: M
      }, m, {
        modifiers: Y({}, s ? {} : {
          // It's using scrollParent by default, we can use the viewport when using a portal.
          preventOverflow: {
            boundariesElement: "window"
          }
        }, c, m.modifiers),
        // We could have been using a custom modifier like react-popper is doing.
        // But it seems this is the best public API for this use case.
        onCreate: Gn(de, m.onCreate),
        onUpdate: Gn(de, m.onUpdate)
      }));
      O.current(Se);
    }
  }, [r, s, c, d, M, m]), q = b.useCallback(function(de) {
    Hr(E, de), F();
  }, [E, F]), z = function() {
    v(!1);
  }, Z = function() {
    L.current && (L.current.destroy(), O.current(null));
  }, Q = function() {
    v(!0), Z();
  };
  if (b.useEffect(function() {
    return function() {
      Z();
    };
  }, []), b.useEffect(function() {
    !d && !j && Z();
  }, [d, j]), !u && !d && (!j || A))
    return null;
  var ue = {
    placement: $
  };
  return j && (ue.TransitionProps = {
    in: d,
    onEnter: z,
    onExited: Q
  }), /* @__PURE__ */ b.createElement(cu, {
    disablePortal: s,
    container: o
  }, /* @__PURE__ */ b.createElement("div", Y({
    ref: q,
    role: "tooltip"
  }, f, {
    style: Y({
      // Prevents scroll issue, waiting for Popper.js to add this style once initiated.
      position: "fixed",
      // Fix Popper.js display issue
      top: 0,
      left: 0,
      display: !d && u && !j ? "none" : null
    }, w)
  }), typeof n == "function" ? n(ue) : n));
});
function Dl(t, e) {
  return Sr(e) === "object" && e !== null ? t === e : String(t) === String(e);
}
function Lv(t) {
  return t == null || typeof t == "string" && !t.trim();
}
var Dv = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e["aria-label"], n = e.autoFocus, o = e.autoWidth, i = e.children, s = e.classes, l = e.className, u = e.defaultValue, c = e.disabled, d = e.displayEmpty, p = e.IconComponent, h = e.inputRef, g = e.labelId, m = e.MenuProps, y = m === void 0 ? {} : m, w = e.multiple, k = e.name, j = e.onBlur, f = e.onChange, x = e.onClose, E = e.onFocus, L = e.onOpen, W = e.open, O = e.readOnly, P = e.renderValue, A = e.SelectDisplayProps, v = A === void 0 ? {} : A, C = e.tabIndex;
  e.type;
  var M = e.value, D = e.variant, $ = D === void 0 ? "standard" : D, H = Ae(e, ["aria-label", "autoFocus", "autoWidth", "children", "classes", "className", "defaultValue", "disabled", "displayEmpty", "IconComponent", "inputRef", "labelId", "MenuProps", "multiple", "name", "onBlur", "onChange", "onClose", "onFocus", "onOpen", "open", "readOnly", "renderValue", "SelectDisplayProps", "tabIndex", "type", "value", "variant"]), F = ns({
    controlled: M,
    default: u,
    name: "Select"
  }), q = En(F, 2), z = q[0], Z = q[1], Q = b.useRef(null), ue = b.useState(null), de = ue[0], Se = ue[1], fe = b.useRef(W != null), ye = fe.current, be = b.useState(), we = be[0], Ee = be[1], ge = b.useState(!1), ke = ge[0], je = ge[1], Te = St(a, h);
  b.useImperativeHandle(Te, function() {
    return {
      focus: function() {
        de.focus();
      },
      node: Q.current,
      value: z
    };
  }, [de, z]), b.useEffect(function() {
    n && de && de.focus();
  }, [n, de]), b.useEffect(function() {
    if (de) {
      var Ce = cr(de).getElementById(g);
      if (Ce) {
        var me = function() {
          getSelection().isCollapsed && de.focus();
        };
        return Ce.addEventListener("click", me), function() {
          Ce.removeEventListener("click", me);
        };
      }
    }
  }, [g, de]);
  var He = function(me, De) {
    me ? L && L(De) : x && x(De), ye || (Ee(o ? null : de.clientWidth), je(me));
  }, _e = function(me) {
    me.button === 0 && (me.preventDefault(), de.focus(), He(!0, me));
  }, Fe = function(me) {
    He(!1, me);
  }, We = b.Children.toArray(i), Ue = function(me) {
    var De = We.map(function(Pe) {
      return Pe.props.value;
    }).indexOf(me.target.value);
    if (De !== -1) {
      var ze = We[De];
      Z(ze.props.value), f && f(me, ze);
    }
  }, tt = function(me) {
    return function(De) {
      w || He(!1, De);
      var ze;
      if (w) {
        ze = Array.isArray(z) ? z.slice() : [];
        var Pe = z.indexOf(me.props.value);
        Pe === -1 ? ze.push(me.props.value) : ze.splice(Pe, 1);
      } else
        ze = me.props.value;
      me.props.onClick && me.props.onClick(De), z !== ze && (Z(ze), f && (De.persist(), Object.defineProperty(De, "target", {
        writable: !0,
        value: {
          value: ze,
          name: k
        }
      }), f(De, me)));
    };
  }, Qe = function(me) {
    if (!O) {
      var De = [
        " ",
        "ArrowUp",
        "ArrowDown",
        // The native select doesn't respond to enter on MacOS, but it's recommended by
        // https://www.w3.org/TR/wai-aria-practices/examples/listbox/listbox-collapsible.html
        "Enter"
      ];
      De.indexOf(me.key) !== -1 && (me.preventDefault(), He(!0, me));
    }
  }, Ve = de !== null && (ye ? W : ke), ce = function(me) {
    !Ve && j && (me.persist(), Object.defineProperty(me, "target", {
      writable: !0,
      value: {
        value: z,
        name: k
      }
    }), j(me));
  };
  delete H["aria-invalid"];
  var pe, X, V = [], U = !1;
  (ls({
    value: z
  }) || d) && (P ? pe = P(z) : U = !0);
  var te = We.map(function(Ce) {
    if (!/* @__PURE__ */ b.isValidElement(Ce))
      return null;
    var me;
    if (w) {
      if (!Array.isArray(z))
        throw new Error(vn(2));
      me = z.some(function(De) {
        return Dl(De, Ce.props.value);
      }), me && U && V.push(Ce.props.children);
    } else
      me = Dl(z, Ce.props.value), me && U && (X = Ce.props.children);
    return /* @__PURE__ */ b.cloneElement(Ce, {
      "aria-selected": me ? "true" : void 0,
      onClick: tt(Ce),
      onKeyUp: function(ze) {
        ze.key === " " && ze.preventDefault(), Ce.props.onKeyUp && Ce.props.onKeyUp(ze);
      },
      role: "option",
      selected: me,
      value: void 0,
      // The value is most likely not a valid HTML attribute.
      "data-value": Ce.props.value
      // Instead, we provide it as a data attribute.
    });
  });
  U && (pe = w ? V.join(", ") : X);
  var oe = we;
  !o && ye && de && (oe = de.clientWidth);
  var xe;
  typeof C < "u" ? xe = C : xe = c ? null : 0;
  var Me = v.id || (k ? "mui-component-select-".concat(k) : void 0);
  return /* @__PURE__ */ b.createElement(b.Fragment, null, /* @__PURE__ */ b.createElement("div", Y({
    className: he(
      s.root,
      // TODO v5: merge root and select
      s.select,
      s.selectMenu,
      s[$],
      l,
      c && s.disabled
    ),
    ref: Se,
    tabIndex: xe,
    role: "button",
    "aria-disabled": c ? "true" : void 0,
    "aria-expanded": Ve ? "true" : void 0,
    "aria-haspopup": "listbox",
    "aria-label": r,
    "aria-labelledby": [g, Me].filter(Boolean).join(" ") || void 0,
    onKeyDown: Qe,
    onMouseDown: c || O ? null : _e,
    onBlur: ce,
    onFocus: E
  }, v, {
    // The id is required for proper a11y
    id: Me
  }), Lv(pe) ? (
    // eslint-disable-next-line react/no-danger
    /* @__PURE__ */ b.createElement("span", {
      dangerouslySetInnerHTML: {
        __html: "&#8203;"
      }
    })
  ) : pe), /* @__PURE__ */ b.createElement("input", Y({
    value: Array.isArray(z) ? z.join(",") : z,
    name: k,
    ref: Q,
    "aria-hidden": !0,
    onChange: Ue,
    tabIndex: -1,
    className: s.nativeInput,
    autoFocus: n
  }, H)), /* @__PURE__ */ b.createElement(p, {
    className: he(s.icon, s["icon".concat(pt($))], Ve && s.iconOpen, c && s.disabled)
  }), /* @__PURE__ */ b.createElement(z0, Y({
    id: "menu-".concat(k || ""),
    anchorEl: de,
    open: Ve,
    onClose: Fe
  }, y, {
    MenuListProps: Y({
      "aria-labelledby": g,
      role: "listbox",
      disableListWrap: !0
    }, y.MenuListProps),
    PaperProps: Y({}, y.PaperProps, {
      style: Y({
        minWidth: oe
      }, y.PaperProps != null ? y.PaperProps.style : null)
    })
  }), te));
}), Bv = vu, Fv = /* @__PURE__ */ b.createElement(pu, null), zv = /* @__PURE__ */ b.createElement(s0, null), Mu = /* @__PURE__ */ b.forwardRef(function t(e, a) {
  var r = e.autoWidth, n = r === void 0 ? !1 : r, o = e.children, i = e.classes, s = e.displayEmpty, l = s === void 0 ? !1 : s, u = e.IconComponent, c = u === void 0 ? gu : u, d = e.id, p = e.input, h = e.inputProps, g = e.label, m = e.labelId, y = e.labelWidth, w = y === void 0 ? 0 : y, k = e.MenuProps, j = e.multiple, f = j === void 0 ? !1 : j, x = e.native, E = x === void 0 ? !1 : x, L = e.onClose, W = e.onOpen, O = e.open, P = e.renderValue, A = e.SelectDisplayProps, v = e.variant, C = v === void 0 ? "standard" : v, M = Ae(e, ["autoWidth", "children", "classes", "displayEmpty", "IconComponent", "id", "input", "inputProps", "label", "labelId", "labelWidth", "MenuProps", "multiple", "native", "onClose", "onOpen", "open", "renderValue", "SelectDisplayProps", "variant"]), D = E ? mu : Dv, $ = ss(), H = to({
    props: e,
    muiFormControl: $,
    states: ["variant"]
  }), F = H.variant || C, q = p || {
    standard: Fv,
    outlined: /* @__PURE__ */ b.createElement(Y0, {
      label: g,
      labelWidth: w
    }),
    filled: zv
  }[F];
  return /* @__PURE__ */ b.cloneElement(q, Y({
    // Most of the logic is implemented in `SelectInput`.
    // The `Select` component is a simple API wrapper to expose something better to play with.
    inputComponent: D,
    inputProps: Y({
      children: o,
      IconComponent: c,
      variant: F,
      type: void 0,
      // We render a select. We can ignore the type provided by the `Input`.
      multiple: f
    }, E ? {
      id: d
    } : {
      autoWidth: n,
      displayEmpty: l,
      labelId: m,
      MenuProps: k,
      onClose: L,
      onOpen: W,
      open: O,
      renderValue: P,
      SelectDisplayProps: Y({
        id: d
      }, A)
    }, h, {
      classes: h ? Qi({
        baseClasses: i,
        newClasses: h.classes,
        Component: t
      }) : i
    }, p ? p.props.inputProps : {}),
    ref: a
  }, M));
});
Mu.muiName = "Select";
const Bl = et(Bv, {
  name: "MuiSelect"
})(Mu);
var Wv = function(e) {
  var a;
  return {
    /* Styles applied to the root element. */
    root: Y({}, e.typography.button, (a = {
      maxWidth: 264,
      minWidth: 72,
      position: "relative",
      boxSizing: "border-box",
      minHeight: 48,
      flexShrink: 0,
      padding: "6px 12px"
    }, Ot(a, e.breakpoints.up("sm"), {
      padding: "6px 24px"
    }), Ot(a, "overflow", "hidden"), Ot(a, "whiteSpace", "normal"), Ot(a, "textAlign", "center"), Ot(a, e.breakpoints.up("sm"), {
      minWidth: 160
    }), a)),
    /* Styles applied to the root element if both `icon` and `label` are provided. */
    labelIcon: {
      minHeight: 72,
      paddingTop: 9,
      "& $wrapper > *:first-child": {
        marginBottom: 6
      }
    },
    /* Styles applied to the root element if the parent [`Tabs`](/api/tabs/) has `textColor="inherit"`. */
    textColorInherit: {
      color: "inherit",
      opacity: 0.7,
      "&$selected": {
        opacity: 1
      },
      "&$disabled": {
        opacity: 0.5
      }
    },
    /* Styles applied to the root element if the parent [`Tabs`](/api/tabs/) has `textColor="primary"`. */
    textColorPrimary: {
      color: e.palette.text.secondary,
      "&$selected": {
        color: e.palette.primary.main
      },
      "&$disabled": {
        color: e.palette.text.disabled
      }
    },
    /* Styles applied to the root element if the parent [`Tabs`](/api/tabs/) has `textColor="secondary"`. */
    textColorSecondary: {
      color: e.palette.text.secondary,
      "&$selected": {
        color: e.palette.secondary.main
      },
      "&$disabled": {
        color: e.palette.text.disabled
      }
    },
    /* Pseudo-class applied to the root element if `selected={true}` (controlled by the Tabs component). */
    selected: {},
    /* Pseudo-class applied to the root element if `disabled={true}` (controlled by the Tabs component). */
    disabled: {},
    /* Styles applied to the root element if `fullWidth={true}` (controlled by the Tabs component). */
    fullWidth: {
      flexShrink: 1,
      flexGrow: 1,
      flexBasis: 0,
      maxWidth: "none"
    },
    /* Styles applied to the root element if `wrapped={true}`. */
    wrapped: {
      fontSize: e.typography.pxToRem(12),
      lineHeight: 1.5
    },
    /* Styles applied to the `icon` and `label`'s wrapper element. */
    wrapper: {
      display: "inline-flex",
      alignItems: "center",
      justifyContent: "center",
      width: "100%",
      flexDirection: "column"
    }
  };
}, Vv = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.classes, n = e.className, o = e.disabled, i = o === void 0 ? !1 : o, s = e.disableFocusRipple, l = s === void 0 ? !1 : s, u = e.fullWidth, c = e.icon, d = e.indicator, p = e.label, h = e.onChange, g = e.onClick, m = e.onFocus, y = e.selected, w = e.selectionFollowsFocus, k = e.textColor, j = k === void 0 ? "inherit" : k, f = e.value, x = e.wrapped, E = x === void 0 ? !1 : x, L = Ae(e, ["classes", "className", "disabled", "disableFocusRipple", "fullWidth", "icon", "indicator", "label", "onChange", "onClick", "onFocus", "selected", "selectionFollowsFocus", "textColor", "value", "wrapped"]), W = function(A) {
    h && h(A, f), g && g(A);
  }, O = function(A) {
    w && !y && h && h(A, f), m && m(A);
  };
  return /* @__PURE__ */ b.createElement(kr, Y({
    focusRipple: !l,
    className: he(r.root, r["textColor".concat(pt(j))], n, i && r.disabled, y && r.selected, p && c && r.labelIcon, u && r.fullWidth, E && r.wrapped),
    ref: a,
    role: "tab",
    "aria-selected": y,
    disabled: i,
    onClick: W,
    onFocus: O,
    tabIndex: y ? 0 : -1
  }, L), /* @__PURE__ */ b.createElement("span", {
    className: r.wrapper
  }, c, p), d);
});
const Hv = et(Wv, {
  name: "MuiTab"
})(Vv), Kv = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M15.41 16.09l-4.58-4.59 4.58-4.59L14 5.5l-6 6 6 6z"
})), Uv = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M8.59 16.34l4.58-4.59-4.58-4.59L10 5.75l6 6-6 6z"
}));
var sn;
function Nu() {
  if (sn)
    return sn;
  var t = document.createElement("div"), e = document.createElement("div");
  return e.style.width = "10px", e.style.height = "1px", t.appendChild(e), t.dir = "rtl", t.style.fontSize = "14px", t.style.width = "4px", t.style.height = "1px", t.style.position = "absolute", t.style.top = "-1000px", t.style.overflow = "scroll", document.body.appendChild(t), sn = "reverse", t.scrollLeft > 0 ? sn = "default" : (t.scrollLeft = 1, t.scrollLeft === 0 && (sn = "negative")), document.body.removeChild(t), sn;
}
function Fl(t, e) {
  var a = t.scrollLeft;
  if (e !== "rtl")
    return a;
  var r = Nu();
  switch (r) {
    case "negative":
      return t.scrollWidth - t.clientWidth + a;
    case "reverse":
      return t.scrollWidth - t.clientWidth - a;
    default:
      return a;
  }
}
function qv(t) {
  return (1 + Math.sin(Math.PI * t - Math.PI / 2)) / 2;
}
function Gv(t, e, a) {
  var r = arguments.length > 3 && arguments[3] !== void 0 ? arguments[3] : {}, n = arguments.length > 4 && arguments[4] !== void 0 ? arguments[4] : function() {
  }, o = r.ease, i = o === void 0 ? qv : o, s = r.duration, l = s === void 0 ? 300 : s, u = null, c = e[t], d = !1, p = function() {
    d = !0;
  }, h = function g(m) {
    if (d) {
      n(new Error("Animation cancelled"));
      return;
    }
    u === null && (u = m);
    var y = Math.min(1, (m - u) / l);
    if (e[t] = i(y) * (a - c) + c, y >= 1) {
      requestAnimationFrame(function() {
        n(null);
      });
      return;
    }
    requestAnimationFrame(g);
  };
  return c === a ? (n(new Error("Element already at target position")), p) : (requestAnimationFrame(h), p);
}
var Yv = {
  width: 99,
  height: 99,
  position: "absolute",
  top: -9999,
  overflow: "scroll"
};
function Xv(t) {
  var e = t.onChange, a = Ae(t, ["onChange"]), r = b.useRef(), n = b.useRef(null), o = function() {
    r.current = n.current.offsetHeight - n.current.clientHeight;
  };
  return b.useEffect(function() {
    var i = Yn(function() {
      var s = r.current;
      o(), s !== r.current && e(r.current);
    });
    return window.addEventListener("resize", i), function() {
      i.clear(), window.removeEventListener("resize", i);
    };
  }, [e]), b.useEffect(function() {
    o(), e(r.current);
  }, [e]), /* @__PURE__ */ b.createElement("div", Y({
    style: Yv,
    ref: n
  }, a));
}
var Jv = function(e) {
  return {
    root: {
      position: "absolute",
      height: 2,
      bottom: 0,
      width: "100%",
      transition: e.transitions.create()
    },
    colorPrimary: {
      backgroundColor: e.palette.primary.main
    },
    colorSecondary: {
      backgroundColor: e.palette.secondary.main
    },
    vertical: {
      height: "100%",
      width: 2,
      right: 0
    }
  };
}, Zv = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.classes, n = e.className, o = e.color, i = e.orientation, s = Ae(e, ["classes", "className", "color", "orientation"]);
  return /* @__PURE__ */ b.createElement("span", Y({
    className: he(r.root, r["color".concat(pt(o))], n, i === "vertical" && r.vertical),
    ref: a
  }, s));
});
const Qv = et(Jv, {
  name: "PrivateTabIndicator"
})(Zv);
var eb = {
  /* Styles applied to the root element. */
  root: {
    width: 40,
    flexShrink: 0,
    opacity: 0.8,
    "&$disabled": {
      opacity: 0
    }
  },
  /* Styles applied to the root element if `orientation="vertical"`. */
  vertical: {
    width: "100%",
    height: 40,
    "& svg": {
      transform: "rotate(90deg)"
    }
  },
  /* Pseudo-class applied to the root element if `disabled={true}`. */
  disabled: {}
}, tb = /* @__PURE__ */ b.createElement(Kv, {
  fontSize: "small"
}), rb = /* @__PURE__ */ b.createElement(Uv, {
  fontSize: "small"
}), nb = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.classes, n = e.className, o = e.direction, i = e.orientation, s = e.disabled, l = Ae(e, ["classes", "className", "direction", "orientation", "disabled"]);
  return /* @__PURE__ */ b.createElement(kr, Y({
    component: "div",
    className: he(r.root, n, s && r.disabled, i === "vertical" && r.vertical),
    ref: a,
    role: null,
    tabIndex: null
  }, l), o === "left" ? tb : rb);
});
const ab = et(eb, {
  name: "MuiTabScrollButton"
})(nb);
var ob = function(e) {
  return {
    /* Styles applied to the root element. */
    root: {
      overflow: "hidden",
      minHeight: 48,
      WebkitOverflowScrolling: "touch",
      // Add iOS momentum scrolling.
      display: "flex"
    },
    /* Styles applied to the root element if `orientation="vertical"`. */
    vertical: {
      flexDirection: "column"
    },
    /* Styles applied to the flex container element. */
    flexContainer: {
      display: "flex"
    },
    /* Styles applied to the flex container element if `orientation="vertical"`. */
    flexContainerVertical: {
      flexDirection: "column"
    },
    /* Styles applied to the flex container element if `centered={true}` & `!variant="scrollable"`. */
    centered: {
      justifyContent: "center"
    },
    /* Styles applied to the tablist element. */
    scroller: {
      position: "relative",
      display: "inline-block",
      flex: "1 1 auto",
      whiteSpace: "nowrap"
    },
    /* Styles applied to the tablist element if `!variant="scrollable"`. */
    fixed: {
      overflowX: "hidden",
      width: "100%"
    },
    /* Styles applied to the tablist element if `variant="scrollable"`. */
    scrollable: {
      overflowX: "scroll",
      // Hide dimensionless scrollbar on MacOS
      scrollbarWidth: "none",
      // Firefox
      "&::-webkit-scrollbar": {
        display: "none"
        // Safari + Chrome
      }
    },
    /* Styles applied to the `ScrollButtonComponent` component. */
    scrollButtons: {},
    /* Styles applied to the `ScrollButtonComponent` component if `scrollButtons="auto"` or scrollButtons="desktop"`. */
    scrollButtonsDesktop: Ot({}, e.breakpoints.down("xs"), {
      display: "none"
    }),
    /* Styles applied to the `TabIndicator` component. */
    indicator: {}
  };
}, ib = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e["aria-label"], n = e["aria-labelledby"], o = e.action, i = e.centered, s = i === void 0 ? !1 : i, l = e.children, u = e.classes, c = e.className, d = e.component, p = d === void 0 ? "div" : d, h = e.indicatorColor, g = h === void 0 ? "secondary" : h, m = e.onChange, y = e.orientation, w = y === void 0 ? "horizontal" : y, k = e.ScrollButtonComponent, j = k === void 0 ? ab : k, f = e.scrollButtons, x = f === void 0 ? "auto" : f, E = e.selectionFollowsFocus, L = e.TabIndicatorProps, W = L === void 0 ? {} : L, O = e.TabScrollButtonProps, P = e.textColor, A = P === void 0 ? "inherit" : P, v = e.value, C = e.variant, M = C === void 0 ? "standard" : C, D = Ae(e, ["aria-label", "aria-labelledby", "action", "centered", "children", "classes", "className", "component", "indicatorColor", "onChange", "orientation", "ScrollButtonComponent", "scrollButtons", "selectionFollowsFocus", "TabIndicatorProps", "TabScrollButtonProps", "textColor", "value", "variant"]), $ = $n(), H = M === "scrollable", F = $.direction === "rtl", q = w === "vertical", z = q ? "scrollTop" : "scrollLeft", Z = q ? "top" : "left", Q = q ? "bottom" : "right", ue = q ? "clientHeight" : "clientWidth", de = q ? "height" : "width", Se = b.useState(!1), fe = Se[0], ye = Se[1], be = b.useState({}), we = be[0], Ee = be[1], ge = b.useState({
    start: !1,
    end: !1
  }), ke = ge[0], je = ge[1], Te = b.useState({
    overflow: "hidden",
    marginBottom: null
  }), He = Te[0], _e = Te[1], Fe = /* @__PURE__ */ new Map(), We = b.useRef(null), Ue = b.useRef(null), tt = function() {
    var ie = We.current, $e;
    if (ie) {
      var qe = ie.getBoundingClientRect();
      $e = {
        clientWidth: ie.clientWidth,
        scrollLeft: ie.scrollLeft,
        scrollTop: ie.scrollTop,
        scrollLeftNormalized: Fl(ie, $.direction),
        scrollWidth: ie.scrollWidth,
        top: qe.top,
        bottom: qe.bottom,
        left: qe.left,
        right: qe.right
      };
    }
    var Je;
    if (ie && v !== !1) {
      var at = Ue.current.children;
      if (at.length > 0) {
        var Ne = at[Fe.get(v)];
        Je = Ne ? Ne.getBoundingClientRect() : null;
      }
    }
    return {
      tabsMeta: $e,
      tabMeta: Je
    };
  }, Qe = rr(function() {
    var Pe, ie = tt(), $e = ie.tabsMeta, qe = ie.tabMeta, Je = 0;
    if (qe && $e)
      if (q)
        Je = qe.top - $e.top + $e.scrollTop;
      else {
        var at = F ? $e.scrollLeftNormalized + $e.clientWidth - $e.scrollWidth : $e.scrollLeft;
        Je = qe.left - $e.left + at;
      }
    var Ne = (Pe = {}, Ot(Pe, Z, Je), Ot(Pe, de, qe ? qe[de] : 0), Pe);
    if (isNaN(we[Z]) || isNaN(we[de]))
      Ee(Ne);
    else {
      var Le = Math.abs(we[Z] - Ne[Z]), mt = Math.abs(we[de] - Ne[de]);
      (Le >= 1 || mt >= 1) && Ee(Ne);
    }
  }), Ve = function(ie) {
    Gv(z, We.current, ie);
  }, ce = function(ie) {
    var $e = We.current[z];
    q ? $e += ie : ($e += ie * (F ? -1 : 1), $e *= F && Nu() === "reverse" ? -1 : 1), Ve($e);
  }, pe = function() {
    ce(-We.current[ue]);
  }, X = function() {
    ce(We.current[ue]);
  }, V = b.useCallback(function(Pe) {
    _e({
      overflow: null,
      marginBottom: -Pe
    });
  }, []), U = function() {
    var ie = {};
    ie.scrollbarSizeListener = H ? /* @__PURE__ */ b.createElement(Xv, {
      className: u.scrollable,
      onChange: V
    }) : null;
    var $e = ke.start || ke.end, qe = H && (x === "auto" && $e || x === "desktop" || x === "on");
    return ie.scrollButtonStart = qe ? /* @__PURE__ */ b.createElement(j, Y({
      orientation: w,
      direction: F ? "right" : "left",
      onClick: pe,
      disabled: !ke.start,
      className: he(u.scrollButtons, x !== "on" && u.scrollButtonsDesktop)
    }, O)) : null, ie.scrollButtonEnd = qe ? /* @__PURE__ */ b.createElement(j, Y({
      orientation: w,
      direction: F ? "left" : "right",
      onClick: X,
      disabled: !ke.end,
      className: he(u.scrollButtons, x !== "on" && u.scrollButtonsDesktop)
    }, O)) : null, ie;
  }, te = rr(function() {
    var Pe = tt(), ie = Pe.tabsMeta, $e = Pe.tabMeta;
    if (!(!$e || !ie)) {
      if ($e[Z] < ie[Z]) {
        var qe = ie[z] + ($e[Z] - ie[Z]);
        Ve(qe);
      } else if ($e[Q] > ie[Q]) {
        var Je = ie[z] + ($e[Q] - ie[Q]);
        Ve(Je);
      }
    }
  }), oe = rr(function() {
    if (H && x !== "off") {
      var Pe = We.current, ie = Pe.scrollTop, $e = Pe.scrollHeight, qe = Pe.clientHeight, Je = Pe.scrollWidth, at = Pe.clientWidth, Ne, Le;
      if (q)
        Ne = ie > 1, Le = ie < $e - qe - 1;
      else {
        var mt = Fl(We.current, $.direction);
        Ne = F ? mt < Je - at - 1 : mt > 1, Le = F ? mt > 1 : mt < Je - at - 1;
      }
      (Ne !== ke.start || Le !== ke.end) && je({
        start: Ne,
        end: Le
      });
    }
  });
  b.useEffect(function() {
    var Pe = Yn(function() {
      Qe(), oe();
    }), ie = rs(We.current);
    return ie.addEventListener("resize", Pe), function() {
      Pe.clear(), ie.removeEventListener("resize", Pe);
    };
  }, [Qe, oe]);
  var xe = b.useCallback(Yn(function() {
    oe();
  }));
  b.useEffect(function() {
    return function() {
      xe.clear();
    };
  }, [xe]), b.useEffect(function() {
    ye(!0);
  }, []), b.useEffect(function() {
    Qe(), oe();
  }), b.useEffect(function() {
    te();
  }, [te, we]), b.useImperativeHandle(o, function() {
    return {
      updateIndicator: Qe,
      updateScrollButtons: oe
    };
  }, [Qe, oe]);
  var Me = /* @__PURE__ */ b.createElement(Qv, Y({
    className: u.indicator,
    orientation: w,
    color: g
  }, W, {
    style: Y({}, we, W.style)
  })), Ce = 0, me = b.Children.map(l, function(Pe) {
    if (!/* @__PURE__ */ b.isValidElement(Pe))
      return null;
    var ie = Pe.props.value === void 0 ? Ce : Pe.props.value;
    Fe.set(ie, Ce);
    var $e = ie === v;
    return Ce += 1, /* @__PURE__ */ b.cloneElement(Pe, {
      fullWidth: M === "fullWidth",
      indicator: $e && !fe && Me,
      selected: $e,
      selectionFollowsFocus: E,
      onChange: m,
      textColor: A,
      value: ie
    });
  }), De = function(ie) {
    var $e = ie.target, qe = $e.getAttribute("role");
    if (qe === "tab") {
      var Je = null, at = w !== "vertical" ? "ArrowLeft" : "ArrowUp", Ne = w !== "vertical" ? "ArrowRight" : "ArrowDown";
      switch (w !== "vertical" && $.direction === "rtl" && (at = "ArrowRight", Ne = "ArrowLeft"), ie.key) {
        case at:
          Je = $e.previousElementSibling || Ue.current.lastChild;
          break;
        case Ne:
          Je = $e.nextElementSibling || Ue.current.firstChild;
          break;
        case "Home":
          Je = Ue.current.firstChild;
          break;
        case "End":
          Je = Ue.current.lastChild;
          break;
      }
      Je !== null && (Je.focus(), ie.preventDefault());
    }
  }, ze = U();
  return /* @__PURE__ */ b.createElement(p, Y({
    className: he(u.root, c, q && u.vertical),
    ref: a
  }, D), ze.scrollButtonStart, ze.scrollbarSizeListener, /* @__PURE__ */ b.createElement("div", {
    className: he(u.scroller, H ? u.scrollable : u.fixed),
    style: He,
    ref: We,
    onScroll: xe
  }, /* @__PURE__ */ b.createElement("div", {
    "aria-label": r,
    "aria-labelledby": n,
    className: he(u.flexContainer, q && u.flexContainerVertical, s && !H && u.centered),
    onKeyDown: De,
    ref: Ue,
    role: "tablist"
  }, me), fe && Me), ze.scrollButtonEnd);
});
const sb = et(ob, {
  name: "MuiTabs"
})(ib);
function zl(t) {
  return Math.round(t * 1e5) / 1e5;
}
function lb() {
  return {
    '&[x-placement*="bottom"] $arrow': {
      top: 0,
      left: 0,
      marginTop: "-0.71em",
      marginLeft: 4,
      marginRight: 4,
      "&::before": {
        transformOrigin: "0 100%"
      }
    },
    '&[x-placement*="top"] $arrow': {
      bottom: 0,
      left: 0,
      marginBottom: "-0.71em",
      marginLeft: 4,
      marginRight: 4,
      "&::before": {
        transformOrigin: "100% 0"
      }
    },
    '&[x-placement*="right"] $arrow': {
      left: 0,
      marginLeft: "-0.71em",
      height: "1em",
      width: "0.71em",
      marginTop: 4,
      marginBottom: 4,
      "&::before": {
        transformOrigin: "100% 100%"
      }
    },
    '&[x-placement*="left"] $arrow': {
      right: 0,
      marginRight: "-0.71em",
      height: "1em",
      width: "0.71em",
      marginTop: 4,
      marginBottom: 4,
      "&::before": {
        transformOrigin: "0 0"
      }
    }
  };
}
var cb = function(e) {
  return {
    /* Styles applied to the Popper component. */
    popper: {
      zIndex: e.zIndex.tooltip,
      pointerEvents: "none"
      // disable jss-rtl plugin
    },
    /* Styles applied to the Popper component if `interactive={true}`. */
    popperInteractive: {
      pointerEvents: "auto"
    },
    /* Styles applied to the Popper component if `arrow={true}`. */
    popperArrow: lb(),
    /* Styles applied to the tooltip (label wrapper) element. */
    tooltip: {
      backgroundColor: Ye(e.palette.grey[700], 0.9),
      borderRadius: e.shape.borderRadius,
      color: e.palette.common.white,
      fontFamily: e.typography.fontFamily,
      padding: "4px 8px",
      fontSize: e.typography.pxToRem(10),
      lineHeight: "".concat(zl(14 / 10), "em"),
      maxWidth: 300,
      wordWrap: "break-word",
      fontWeight: e.typography.fontWeightMedium
    },
    /* Styles applied to the tooltip (label wrapper) element if `arrow={true}`. */
    tooltipArrow: {
      position: "relative",
      margin: "0"
    },
    /* Styles applied to the arrow element. */
    arrow: {
      overflow: "hidden",
      position: "absolute",
      width: "1em",
      height: "0.71em",
      boxSizing: "border-box",
      color: Ye(e.palette.grey[700], 0.9),
      "&::before": {
        content: '""',
        margin: "auto",
        display: "block",
        width: "100%",
        height: "100%",
        backgroundColor: "currentColor",
        transform: "rotate(45deg)"
      }
    },
    /* Styles applied to the tooltip (label wrapper) element if the tooltip is opened by touch. */
    touch: {
      padding: "8px 16px",
      fontSize: e.typography.pxToRem(14),
      lineHeight: "".concat(zl(16 / 14), "em"),
      fontWeight: e.typography.fontWeightRegular
    },
    /* Styles applied to the tooltip (label wrapper) element if `placement` contains "left". */
    tooltipPlacementLeft: Ot({
      transformOrigin: "right center",
      margin: "0 24px "
    }, e.breakpoints.up("sm"), {
      margin: "0 14px"
    }),
    /* Styles applied to the tooltip (label wrapper) element if `placement` contains "right". */
    tooltipPlacementRight: Ot({
      transformOrigin: "left center",
      margin: "0 24px"
    }, e.breakpoints.up("sm"), {
      margin: "0 14px"
    }),
    /* Styles applied to the tooltip (label wrapper) element if `placement` contains "top". */
    tooltipPlacementTop: Ot({
      transformOrigin: "center bottom",
      margin: "24px 0"
    }, e.breakpoints.up("sm"), {
      margin: "14px 0"
    }),
    /* Styles applied to the tooltip (label wrapper) element if `placement` contains "bottom". */
    tooltipPlacementBottom: Ot({
      transformOrigin: "center top",
      margin: "24px 0"
    }, e.breakpoints.up("sm"), {
      margin: "14px 0"
    })
  };
}, Ca = !1, ai = null, ub = /* @__PURE__ */ b.forwardRef(function(e, a) {
  var r = e.arrow, n = r === void 0 ? !1 : r, o = e.children, i = e.classes, s = e.disableFocusListener, l = s === void 0 ? !1 : s, u = e.disableHoverListener, c = u === void 0 ? !1 : u, d = e.disableTouchListener, p = d === void 0 ? !1 : d, h = e.enterDelay, g = h === void 0 ? 100 : h, m = e.enterNextDelay, y = m === void 0 ? 0 : m, w = e.enterTouchDelay, k = w === void 0 ? 700 : w, j = e.id, f = e.interactive, x = f === void 0 ? !1 : f, E = e.leaveDelay, L = E === void 0 ? 0 : E, W = e.leaveTouchDelay, O = W === void 0 ? 1500 : W, P = e.onClose, A = e.onOpen, v = e.open, C = e.placement, M = C === void 0 ? "bottom" : C, D = e.PopperComponent, $ = D === void 0 ? Nv : D, H = e.PopperProps, F = e.title, q = e.TransitionComponent, z = q === void 0 ? cs : q, Z = e.TransitionProps, Q = Ae(e, ["arrow", "children", "classes", "disableFocusListener", "disableHoverListener", "disableTouchListener", "enterDelay", "enterNextDelay", "enterTouchDelay", "id", "interactive", "leaveDelay", "leaveTouchDelay", "onClose", "onOpen", "open", "placement", "PopperComponent", "PopperProps", "title", "TransitionComponent", "TransitionProps"]), ue = $n(), de = b.useState(), Se = de[0], fe = de[1], ye = b.useState(null), be = ye[0], we = ye[1], Ee = b.useRef(!1), ge = b.useRef(), ke = b.useRef(), je = b.useRef(), Te = b.useRef(), He = ns({
    controlled: v,
    default: !1,
    name: "Tooltip",
    state: "open"
  }), _e = En(He, 2), Fe = _e[0], We = _e[1], Ue = Fe, tt = zm(j);
  b.useEffect(function() {
    return function() {
      clearTimeout(ge.current), clearTimeout(ke.current), clearTimeout(je.current), clearTimeout(Te.current);
    };
  }, []);
  var Qe = function(rt) {
    clearTimeout(ai), Ca = !0, We(!0), A && A(rt);
  }, Ve = function() {
    var rt = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : !0;
    return function(Ze) {
      var bt = o.props;
      Ze.type === "mouseover" && bt.onMouseOver && rt && bt.onMouseOver(Ze), !(Ee.current && Ze.type !== "touchstart") && (Se && Se.removeAttribute("title"), clearTimeout(ke.current), clearTimeout(je.current), g || Ca && y ? (Ze.persist(), ke.current = setTimeout(function() {
        Qe(Ze);
      }, Ca ? y : g)) : Qe(Ze));
    };
  }, ce = as(), pe = ce.isFocusVisible, X = ce.onBlurVisible, V = ce.ref, U = b.useState(!1), te = U[0], oe = U[1], xe = function() {
    te && (oe(!1), X());
  }, Me = function() {
    var rt = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : !0;
    return function(Ze) {
      Se || fe(Ze.currentTarget), pe(Ze) && (oe(!0), Ve()(Ze));
      var bt = o.props;
      bt.onFocus && rt && bt.onFocus(Ze);
    };
  }, Ce = function(rt) {
    clearTimeout(ai), ai = setTimeout(function() {
      Ca = !1;
    }, 800 + L), We(!1), P && P(rt), clearTimeout(ge.current), ge.current = setTimeout(function() {
      Ee.current = !1;
    }, ue.transitions.duration.shortest);
  }, me = function() {
    var rt = arguments.length > 0 && arguments[0] !== void 0 ? arguments[0] : !0;
    return function(Ze) {
      var bt = o.props;
      Ze.type === "blur" && (bt.onBlur && rt && bt.onBlur(Ze), xe()), Ze.type === "mouseleave" && bt.onMouseLeave && Ze.currentTarget === Se && bt.onMouseLeave(Ze), clearTimeout(ke.current), clearTimeout(je.current), Ze.persist(), je.current = setTimeout(function() {
        Ce(Ze);
      }, L);
    };
  }, De = function(rt) {
    Ee.current = !0;
    var Ze = o.props;
    Ze.onTouchStart && Ze.onTouchStart(rt);
  }, ze = function(rt) {
    De(rt), clearTimeout(je.current), clearTimeout(ge.current), clearTimeout(Te.current), rt.persist(), Te.current = setTimeout(function() {
      Ve()(rt);
    }, k);
  }, Pe = function(rt) {
    o.props.onTouchEnd && o.props.onTouchEnd(rt), clearTimeout(Te.current), clearTimeout(je.current), rt.persist(), je.current = setTimeout(function() {
      Ce(rt);
    }, O);
  }, ie = St(fe, a), $e = St(V, ie), qe = b.useCallback(function(ot) {
    Hr($e, Ut.findDOMNode(ot));
  }, [$e]), Je = St(o.ref, qe);
  F === "" && (Ue = !1);
  var at = !Ue && !c, Ne = Y({
    "aria-describedby": Ue ? tt : null,
    title: at && typeof F == "string" ? F : null
  }, Q, o.props, {
    className: he(Q.className, o.props.className),
    onTouchStart: De,
    ref: Je
  }), Le = {};
  p || (Ne.onTouchStart = ze, Ne.onTouchEnd = Pe), c || (Ne.onMouseOver = Ve(), Ne.onMouseLeave = me(), x && (Le.onMouseOver = Ve(!1), Le.onMouseLeave = me(!1))), l || (Ne.onFocus = Me(), Ne.onBlur = me(), x && (Le.onFocus = Me(!1), Le.onBlur = me(!1)));
  var mt = b.useMemo(function() {
    return wr({
      popperOptions: {
        modifiers: {
          arrow: {
            enabled: !!be,
            element: be
          }
        }
      }
    }, H);
  }, [be, H]);
  return /* @__PURE__ */ b.createElement(b.Fragment, null, /* @__PURE__ */ b.cloneElement(o, Ne), /* @__PURE__ */ b.createElement($, Y({
    className: he(i.popper, x && i.popperInteractive, n && i.popperArrow),
    placement: M,
    anchorEl: Se,
    open: Se ? Ue : !1,
    id: Ne["aria-describedby"],
    transition: !0
  }, Le, mt), function(ot) {
    var rt = ot.placement, Ze = ot.TransitionProps;
    return /* @__PURE__ */ b.createElement(z, Y({
      timeout: ue.transitions.duration.shorter
    }, Ze, Z), /* @__PURE__ */ b.createElement("div", {
      className: he(i.tooltip, i["tooltipPlacement".concat(pt(rt.split("-")[0]))], Ee.current && i.touch, n && i.tooltipArrow)
    }, F, n ? /* @__PURE__ */ b.createElement("span", {
      className: i.arrow,
      ref: we
    }) : null));
  }));
});
const ms = et(cb, {
  name: "MuiTooltip",
  flip: !1
})(ub);
var Fn = { exports: {} };
Fn.exports;
var Wl;
function db() {
  return Wl || (Wl = 1, (function(t, e) {
    var a = 200, r = "__lodash_hash_undefined__", n = 9007199254740991, o = "[object Arguments]", i = "[object Array]", s = "[object Boolean]", l = "[object Date]", u = "[object Error]", c = "[object Function]", d = "[object GeneratorFunction]", p = "[object Map]", h = "[object Number]", g = "[object Object]", m = "[object Promise]", y = "[object RegExp]", w = "[object Set]", k = "[object String]", j = "[object Symbol]", f = "[object WeakMap]", x = "[object ArrayBuffer]", E = "[object DataView]", L = "[object Float32Array]", W = "[object Float64Array]", O = "[object Int8Array]", P = "[object Int16Array]", A = "[object Int32Array]", v = "[object Uint8Array]", C = "[object Uint8ClampedArray]", M = "[object Uint16Array]", D = "[object Uint32Array]", $ = /[\\^$.*+?()[\]{}|]/g, H = /\w*$/, F = /^\[object .+?Constructor\]$/, q = /^(?:0|[1-9]\d*)$/, z = {};
    z[o] = z[i] = z[x] = z[E] = z[s] = z[l] = z[L] = z[W] = z[O] = z[P] = z[A] = z[p] = z[h] = z[g] = z[y] = z[w] = z[k] = z[j] = z[v] = z[C] = z[M] = z[D] = !0, z[u] = z[c] = z[f] = !1;
    var Z = typeof va == "object" && va && va.Object === Object && va, Q = typeof self == "object" && self && self.Object === Object && self, ue = Z || Q || Function("return this")(), de = e && !e.nodeType && e, Se = de && !0 && t && !t.nodeType && t, fe = Se && Se.exports === de;
    function ye(T, J) {
      return T.set(J[0], J[1]), T;
    }
    function be(T, J) {
      return T.add(J), T;
    }
    function we(T, J) {
      for (var le = -1, Ie = T ? T.length : 0; ++le < Ie && J(T[le], le, T) !== !1; )
        ;
      return T;
    }
    function Ee(T, J) {
      for (var le = -1, Ie = J.length, Pt = T.length; ++le < Ie; )
        T[Pt + le] = J[le];
      return T;
    }
    function ge(T, J, le, Ie) {
      for (var Pt = -1, Dt = T ? T.length : 0; ++Pt < Dt; )
        le = J(le, T[Pt], Pt, T);
      return le;
    }
    function ke(T, J) {
      for (var le = -1, Ie = Array(T); ++le < T; )
        Ie[le] = J(le);
      return Ie;
    }
    function je(T, J) {
      return T == null ? void 0 : T[J];
    }
    function Te(T) {
      var J = !1;
      if (T != null && typeof T.toString != "function")
        try {
          J = !!(T + "");
        } catch {
        }
      return J;
    }
    function He(T) {
      var J = -1, le = Array(T.size);
      return T.forEach(function(Ie, Pt) {
        le[++J] = [Pt, Ie];
      }), le;
    }
    function _e(T, J) {
      return function(le) {
        return T(J(le));
      };
    }
    function Fe(T) {
      var J = -1, le = Array(T.size);
      return T.forEach(function(Ie) {
        le[++J] = Ie;
      }), le;
    }
    var We = Array.prototype, Ue = Function.prototype, tt = Object.prototype, Qe = ue["__core-js_shared__"], Ve = (function() {
      var T = /[^.]+$/.exec(Qe && Qe.keys && Qe.keys.IE_PROTO || "");
      return T ? "Symbol(src)_1." + T : "";
    })(), ce = Ue.toString, pe = tt.hasOwnProperty, X = tt.toString, V = RegExp(
      "^" + ce.call(pe).replace($, "\\$&").replace(/hasOwnProperty|(function).*?(?=\\\()| for .+?(?=\\\])/g, "$1.*?") + "$"
    ), U = fe ? ue.Buffer : void 0, te = ue.Symbol, oe = ue.Uint8Array, xe = _e(Object.getPrototypeOf, Object), Me = Object.create, Ce = tt.propertyIsEnumerable, me = We.splice, De = Object.getOwnPropertySymbols, ze = U ? U.isBuffer : void 0, Pe = _e(Object.keys, Object), ie = Qr(ue, "DataView"), $e = Qr(ue, "Map"), qe = Qr(ue, "Promise"), Je = Qr(ue, "Set"), at = Qr(ue, "WeakMap"), Ne = Qr(Object, "create"), Le = Ir(ie), mt = Ir($e), ot = Ir(qe), rt = Ir(Je), Ze = Ir(at), bt = te ? te.prototype : void 0, Nt = bt ? bt.valueOf : void 0;
    function Lt(T) {
      var J = -1, le = T ? T.length : 0;
      for (this.clear(); ++J < le; ) {
        var Ie = T[J];
        this.set(Ie[0], Ie[1]);
      }
    }
    function _r() {
      this.__data__ = Ne ? Ne(null) : {};
    }
    function xo(T) {
      return this.has(T) && delete this.__data__[T];
    }
    function qr(T) {
      var J = this.__data__;
      if (Ne) {
        var le = J[T];
        return le === r ? void 0 : le;
      }
      return pe.call(J, T) ? J[T] : void 0;
    }
    function Co(T) {
      var J = this.__data__;
      return Ne ? J[T] !== void 0 : pe.call(J, T);
    }
    function So(T, J) {
      var le = this.__data__;
      return le[T] = Ne && J === void 0 ? r : J, this;
    }
    Lt.prototype.clear = _r, Lt.prototype.delete = xo, Lt.prototype.get = qr, Lt.prototype.has = Co, Lt.prototype.set = So;
    function Gt(T) {
      var J = -1, le = T ? T.length : 0;
      for (this.clear(); ++J < le; ) {
        var Ie = T[J];
        this.set(Ie[0], Ie[1]);
      }
    }
    function wo() {
      this.__data__ = [];
    }
    function Gr(T) {
      var J = this.__data__, le = Be(J, T);
      if (le < 0)
        return !1;
      var Ie = J.length - 1;
      return le == Ie ? J.pop() : me.call(J, le, 1), !0;
    }
    function I(T) {
      var J = this.__data__, le = Be(J, T);
      return le < 0 ? void 0 : J[le][1];
    }
    function N(T) {
      return Be(this.__data__, T) > -1;
    }
    function S(T, J) {
      var le = this.__data__, Ie = Be(le, T);
      return Ie < 0 ? le.push([T, J]) : le[Ie][1] = J, this;
    }
    Gt.prototype.clear = wo, Gt.prototype.delete = Gr, Gt.prototype.get = I, Gt.prototype.has = N, Gt.prototype.set = S;
    function R(T) {
      var J = -1, le = T ? T.length : 0;
      for (this.clear(); ++J < le; ) {
        var Ie = T[J];
        this.set(Ie[0], Ie[1]);
      }
    }
    function _() {
      this.__data__ = {
        hash: new Lt(),
        map: new ($e || Gt)(),
        string: new Lt()
      };
    }
    function G(T) {
      return pa(this, T).delete(T);
    }
    function re(T) {
      return pa(this, T).get(T);
    }
    function ee(T) {
      return pa(this, T).has(T);
    }
    function K(T, J) {
      return pa(this, T).set(T, J), this;
    }
    R.prototype.clear = _, R.prototype.delete = G, R.prototype.get = re, R.prototype.has = ee, R.prototype.set = K;
    function se(T) {
      this.__data__ = new Gt(T);
    }
    function Oe() {
      this.__data__ = new Gt();
    }
    function Xe(T) {
      return this.__data__.delete(T);
    }
    function nt(T) {
      return this.__data__.get(T);
    }
    function lt(T) {
      return this.__data__.has(T);
    }
    function Ge(T, J) {
      var le = this.__data__;
      if (le instanceof Gt) {
        var Ie = le.__data__;
        if (!$e || Ie.length < a - 1)
          return Ie.push([T, J]), this;
        le = this.__data__ = new R(Ie);
      }
      return le.set(T, J), this;
    }
    se.prototype.clear = Oe, se.prototype.delete = Xe, se.prototype.get = nt, se.prototype.has = lt, se.prototype.set = Ge;
    function ct(T, J) {
      var le = Eo(T) || $d(T) ? ke(T.length, String) : [], Ie = le.length, Pt = !!Ie;
      for (var Dt in T)
        pe.call(T, Dt) && !(Pt && (Dt == "length" || Sd(Dt, Ie))) && le.push(Dt);
      return le;
    }
    function yt(T, J, le) {
      var Ie = T[J];
      (!(pe.call(T, J) && Os(Ie, le)) || le === void 0 && !(J in T)) && (T[J] = le);
    }
    function Be(T, J) {
      for (var le = T.length; le--; )
        if (Os(T[le][0], J))
          return le;
      return -1;
    }
    function $t(T, J) {
      return T && Zr(J, ko(J), T);
    }
    function _t(T, J, le, Ie, Pt, Dt, ir) {
      var Bt;
      if (Ie && (Bt = Dt ? Ie(T, Pt, Dt, ir) : Ie(T)), Bt !== void 0)
        return Bt;
      if (!ha(T))
        return T;
      var js = Eo(T);
      if (js) {
        if (Bt = yd(T), !J)
          return Pn(T, Bt);
      } else {
        var en = jr(T), Is = en == c || en == d;
        if (Td(T))
          return Tt(T, J);
        if (en == g || en == o || Is && !Dt) {
          if (Te(T))
            return Dt ? T : {};
          if (Bt = xd(Is ? {} : T), !J)
            return mr(T, $t(Bt, T));
        } else {
          if (!z[en])
            return Dt ? T : {};
          Bt = Cd(T, en, _t, J);
        }
      }
      ir || (ir = new se());
      var As = ir.get(T);
      if (As)
        return As;
      if (ir.set(T, Bt), !js)
        var Ms = le ? fa(T) : ko(T);
      return we(Ms || T, function($o, ma) {
        Ms && (ma = $o, $o = T[ma]), yt(Bt, ma, _t($o, J, le, Ie, ma, T, ir));
      }), Bt;
    }
    function Yt(T) {
      return ha(T) ? Me(T) : {};
    }
    function Yr(T, J, le) {
      var Ie = J(T);
      return Eo(T) ? Ie : Ee(Ie, le(T));
    }
    function Xt(T) {
      return X.call(T);
    }
    function Xr(T) {
      if (!ha(T) || Ed(T))
        return !1;
      var J = Ps(T) || Te(T) ? V : F;
      return J.test(Ir(T));
    }
    function pr(T) {
      if (!Ts(T))
        return Pe(T);
      var J = [];
      for (var le in Object(T))
        pe.call(T, le) && le != "constructor" && J.push(le);
      return J;
    }
    function Tt(T, J) {
      if (J)
        return T.slice();
      var le = new T.constructor(T.length);
      return T.copy(le), le;
    }
    function Zt(T) {
      var J = new T.constructor(T.byteLength);
      return new oe(J).set(new oe(T)), J;
    }
    function Pr(T, J) {
      var le = J ? Zt(T.buffer) : T.buffer;
      return new T.constructor(le, T.byteOffset, T.byteLength);
    }
    function hr(T, J, le) {
      var Ie = J ? le(He(T), !0) : He(T);
      return ge(Ie, ye, new T.constructor());
    }
    function da(T) {
      var J = new T.constructor(T.source, H.exec(T));
      return J.lastIndex = T.lastIndex, J;
    }
    function Jr(T, J, le) {
      var Ie = J ? le(Fe(T), !0) : Fe(T);
      return ge(Ie, be, new T.constructor());
    }
    function On(T) {
      return Nt ? Object(Nt.call(T)) : {};
    }
    function _n(T, J) {
      var le = J ? Zt(T.buffer) : T.buffer;
      return new T.constructor(le, T.byteOffset, T.length);
    }
    function Pn(T, J) {
      var le = -1, Ie = T.length;
      for (J || (J = Array(Ie)); ++le < Ie; )
        J[le] = T[le];
      return J;
    }
    function Zr(T, J, le, Ie) {
      le || (le = {});
      for (var Pt = -1, Dt = J.length; ++Pt < Dt; ) {
        var ir = J[Pt], Bt = void 0;
        yt(le, ir, Bt === void 0 ? T[ir] : Bt);
      }
      return le;
    }
    function mr(T, J) {
      return Zr(T, Rs(T), J);
    }
    function fa(T) {
      return Yr(T, ko, Rs);
    }
    function pa(T, J) {
      var le = T.__data__;
      return wd(J) ? le[typeof J == "string" ? "string" : "hash"] : le.map;
    }
    function Qr(T, J) {
      var le = je(T, J);
      return Xr(le) ? le : void 0;
    }
    var Rs = De ? _e(De, Object) : Pd, jr = Xt;
    (ie && jr(new ie(new ArrayBuffer(1))) != E || $e && jr(new $e()) != p || qe && jr(qe.resolve()) != m || Je && jr(new Je()) != w || at && jr(new at()) != f) && (jr = function(T) {
      var J = X.call(T), le = J == g ? T.constructor : void 0, Ie = le ? Ir(le) : void 0;
      if (Ie)
        switch (Ie) {
          case Le:
            return E;
          case mt:
            return p;
          case ot:
            return m;
          case rt:
            return w;
          case Ze:
            return f;
        }
      return J;
    });
    function yd(T) {
      var J = T.length, le = T.constructor(J);
      return J && typeof T[0] == "string" && pe.call(T, "index") && (le.index = T.index, le.input = T.input), le;
    }
    function xd(T) {
      return typeof T.constructor == "function" && !Ts(T) ? Yt(xe(T)) : {};
    }
    function Cd(T, J, le, Ie) {
      var Pt = T.constructor;
      switch (J) {
        case x:
          return Zt(T);
        case s:
        case l:
          return new Pt(+T);
        case E:
          return Pr(T, Ie);
        case L:
        case W:
        case O:
        case P:
        case A:
        case v:
        case C:
        case M:
        case D:
          return _n(T, Ie);
        case p:
          return hr(T, Ie, le);
        case h:
        case k:
          return new Pt(T);
        case y:
          return da(T);
        case w:
          return Jr(T, Ie, le);
        case j:
          return On(T);
      }
    }
    function Sd(T, J) {
      return J = J ?? n, !!J && (typeof T == "number" || q.test(T)) && T > -1 && T % 1 == 0 && T < J;
    }
    function wd(T) {
      var J = typeof T;
      return J == "string" || J == "number" || J == "symbol" || J == "boolean" ? T !== "__proto__" : T === null;
    }
    function Ed(T) {
      return !!Ve && Ve in T;
    }
    function Ts(T) {
      var J = T && T.constructor, le = typeof J == "function" && J.prototype || tt;
      return T === le;
    }
    function Ir(T) {
      if (T != null) {
        try {
          return ce.call(T);
        } catch {
        }
        try {
          return T + "";
        } catch {
        }
      }
      return "";
    }
    function kd(T) {
      return _t(T, !0, !0);
    }
    function Os(T, J) {
      return T === J || T !== T && J !== J;
    }
    function $d(T) {
      return Rd(T) && pe.call(T, "callee") && (!Ce.call(T, "callee") || X.call(T) == o);
    }
    var Eo = Array.isArray;
    function _s(T) {
      return T != null && Od(T.length) && !Ps(T);
    }
    function Rd(T) {
      return _d(T) && _s(T);
    }
    var Td = ze || jd;
    function Ps(T) {
      var J = ha(T) ? X.call(T) : "";
      return J == c || J == d;
    }
    function Od(T) {
      return typeof T == "number" && T > -1 && T % 1 == 0 && T <= n;
    }
    function ha(T) {
      var J = typeof T;
      return !!T && (J == "object" || J == "function");
    }
    function _d(T) {
      return !!T && typeof T == "object";
    }
    function ko(T) {
      return _s(T) ? ct(T) : pr(T);
    }
    function Pd() {
      return [];
    }
    function jd() {
      return !1;
    }
    t.exports = kd;
  })(Fn, Fn.exports)), Fn.exports;
}
var fb = db();
const Vl = /* @__PURE__ */ Tr(fb);
var _a = { exports: {} }, pb = _a.exports, Hl;
function hb() {
  return Hl || (Hl = 1, (function(t, e) {
    (function(a, r) {
      t.exports = r();
    })(pb, (function() {
      var a = 1e3, r = 6e4, n = 36e5, o = "millisecond", i = "second", s = "minute", l = "hour", u = "day", c = "week", d = "month", p = "quarter", h = "year", g = "date", m = "Invalid Date", y = /^(\d{4})[-/]?(\d{1,2})?[-/]?(\d{0,2})[Tt\s]*(\d{1,2})?:?(\d{1,2})?:?(\d{1,2})?[.:]?(\d+)?$/, w = /\[([^\]]+)]|Y{1,4}|M{1,4}|D{1,2}|d{1,4}|H{1,2}|h{1,2}|a|A|m{1,2}|s{1,2}|Z{1,2}|SSS/g, k = { name: "en", weekdays: "Sunday_Monday_Tuesday_Wednesday_Thursday_Friday_Saturday".split("_"), months: "January_February_March_April_May_June_July_August_September_October_November_December".split("_"), ordinal: function(M) {
        var D = ["th", "st", "nd", "rd"], $ = M % 100;
        return "[" + M + (D[($ - 20) % 10] || D[$] || D[0]) + "]";
      } }, j = function(M, D, $) {
        var H = String(M);
        return !H || H.length >= D ? M : "" + Array(D + 1 - H.length).join($) + M;
      }, f = { s: j, z: function(M) {
        var D = -M.utcOffset(), $ = Math.abs(D), H = Math.floor($ / 60), F = $ % 60;
        return (D <= 0 ? "+" : "-") + j(H, 2, "0") + ":" + j(F, 2, "0");
      }, m: function M(D, $) {
        if (D.date() < $.date()) return -M($, D);
        var H = 12 * ($.year() - D.year()) + ($.month() - D.month()), F = D.clone().add(H, d), q = $ - F < 0, z = D.clone().add(H + (q ? -1 : 1), d);
        return +(-(H + ($ - F) / (q ? F - z : z - F)) || 0);
      }, a: function(M) {
        return M < 0 ? Math.ceil(M) || 0 : Math.floor(M);
      }, p: function(M) {
        return { M: d, y: h, w: c, d: u, D: g, h: l, m: s, s: i, ms: o, Q: p }[M] || String(M || "").toLowerCase().replace(/s$/, "");
      }, u: function(M) {
        return M === void 0;
      } }, x = "en", E = {};
      E[x] = k;
      var L = "$isDayjsObject", W = function(M) {
        return M instanceof v || !(!M || !M[L]);
      }, O = function M(D, $, H) {
        var F;
        if (!D) return x;
        if (typeof D == "string") {
          var q = D.toLowerCase();
          E[q] && (F = q), $ && (E[q] = $, F = q);
          var z = D.split("-");
          if (!F && z.length > 1) return M(z[0]);
        } else {
          var Z = D.name;
          E[Z] = D, F = Z;
        }
        return !H && F && (x = F), F || !H && x;
      }, P = function(M, D) {
        if (W(M)) return M.clone();
        var $ = typeof D == "object" ? D : {};
        return $.date = M, $.args = arguments, new v($);
      }, A = f;
      A.l = O, A.i = W, A.w = function(M, D) {
        return P(M, { locale: D.$L, utc: D.$u, x: D.$x, $offset: D.$offset });
      };
      var v = (function() {
        function M($) {
          this.$L = O($.locale, null, !0), this.parse($), this.$x = this.$x || $.x || {}, this[L] = !0;
        }
        var D = M.prototype;
        return D.parse = function($) {
          this.$d = (function(H) {
            var F = H.date, q = H.utc;
            if (F === null) return /* @__PURE__ */ new Date(NaN);
            if (A.u(F)) return /* @__PURE__ */ new Date();
            if (F instanceof Date) return new Date(F);
            if (typeof F == "string" && !/Z$/i.test(F)) {
              var z = F.match(y);
              if (z) {
                var Z = z[2] - 1 || 0, Q = (z[7] || "0").substring(0, 3);
                return q ? new Date(Date.UTC(z[1], Z, z[3] || 1, z[4] || 0, z[5] || 0, z[6] || 0, Q)) : new Date(z[1], Z, z[3] || 1, z[4] || 0, z[5] || 0, z[6] || 0, Q);
              }
            }
            return new Date(F);
          })($), this.init();
        }, D.init = function() {
          var $ = this.$d;
          this.$y = $.getFullYear(), this.$M = $.getMonth(), this.$D = $.getDate(), this.$W = $.getDay(), this.$H = $.getHours(), this.$m = $.getMinutes(), this.$s = $.getSeconds(), this.$ms = $.getMilliseconds();
        }, D.$utils = function() {
          return A;
        }, D.isValid = function() {
          return this.$d.toString() !== m;
        }, D.isSame = function($, H) {
          var F = P($);
          return this.startOf(H) <= F && F <= this.endOf(H);
        }, D.isAfter = function($, H) {
          return P($) < this.startOf(H);
        }, D.isBefore = function($, H) {
          return this.endOf(H) < P($);
        }, D.$g = function($, H, F) {
          return A.u($) ? this[H] : this.set(F, $);
        }, D.unix = function() {
          return Math.floor(this.valueOf() / 1e3);
        }, D.valueOf = function() {
          return this.$d.getTime();
        }, D.startOf = function($, H) {
          var F = this, q = !!A.u(H) || H, z = A.p($), Z = function(we, Ee) {
            var ge = A.w(F.$u ? Date.UTC(F.$y, Ee, we) : new Date(F.$y, Ee, we), F);
            return q ? ge : ge.endOf(u);
          }, Q = function(we, Ee) {
            return A.w(F.toDate()[we].apply(F.toDate("s"), (q ? [0, 0, 0, 0] : [23, 59, 59, 999]).slice(Ee)), F);
          }, ue = this.$W, de = this.$M, Se = this.$D, fe = "set" + (this.$u ? "UTC" : "");
          switch (z) {
            case h:
              return q ? Z(1, 0) : Z(31, 11);
            case d:
              return q ? Z(1, de) : Z(0, de + 1);
            case c:
              var ye = this.$locale().weekStart || 0, be = (ue < ye ? ue + 7 : ue) - ye;
              return Z(q ? Se - be : Se + (6 - be), de);
            case u:
            case g:
              return Q(fe + "Hours", 0);
            case l:
              return Q(fe + "Minutes", 1);
            case s:
              return Q(fe + "Seconds", 2);
            case i:
              return Q(fe + "Milliseconds", 3);
            default:
              return this.clone();
          }
        }, D.endOf = function($) {
          return this.startOf($, !1);
        }, D.$set = function($, H) {
          var F, q = A.p($), z = "set" + (this.$u ? "UTC" : ""), Z = (F = {}, F[u] = z + "Date", F[g] = z + "Date", F[d] = z + "Month", F[h] = z + "FullYear", F[l] = z + "Hours", F[s] = z + "Minutes", F[i] = z + "Seconds", F[o] = z + "Milliseconds", F)[q], Q = q === u ? this.$D + (H - this.$W) : H;
          if (q === d || q === h) {
            var ue = this.clone().set(g, 1);
            ue.$d[Z](Q), ue.init(), this.$d = ue.set(g, Math.min(this.$D, ue.daysInMonth())).$d;
          } else Z && this.$d[Z](Q);
          return this.init(), this;
        }, D.set = function($, H) {
          return this.clone().$set($, H);
        }, D.get = function($) {
          return this[A.p($)]();
        }, D.add = function($, H) {
          var F, q = this;
          $ = Number($);
          var z = A.p(H), Z = function(de) {
            var Se = P(q);
            return A.w(Se.date(Se.date() + Math.round(de * $)), q);
          };
          if (z === d) return this.set(d, this.$M + $);
          if (z === h) return this.set(h, this.$y + $);
          if (z === u) return Z(1);
          if (z === c) return Z(7);
          var Q = (F = {}, F[s] = r, F[l] = n, F[i] = a, F)[z] || 1, ue = this.$d.getTime() + $ * Q;
          return A.w(ue, this);
        }, D.subtract = function($, H) {
          return this.add(-1 * $, H);
        }, D.format = function($) {
          var H = this, F = this.$locale();
          if (!this.isValid()) return F.invalidDate || m;
          var q = $ || "YYYY-MM-DDTHH:mm:ssZ", z = A.z(this), Z = this.$H, Q = this.$m, ue = this.$M, de = F.weekdays, Se = F.months, fe = F.meridiem, ye = function(Ee, ge, ke, je) {
            return Ee && (Ee[ge] || Ee(H, q)) || ke[ge].slice(0, je);
          }, be = function(Ee) {
            return A.s(Z % 12 || 12, Ee, "0");
          }, we = fe || function(Ee, ge, ke) {
            var je = Ee < 12 ? "AM" : "PM";
            return ke ? je.toLowerCase() : je;
          };
          return q.replace(w, (function(Ee, ge) {
            return ge || (function(ke) {
              switch (ke) {
                case "YY":
                  return String(H.$y).slice(-2);
                case "YYYY":
                  return A.s(H.$y, 4, "0");
                case "M":
                  return ue + 1;
                case "MM":
                  return A.s(ue + 1, 2, "0");
                case "MMM":
                  return ye(F.monthsShort, ue, Se, 3);
                case "MMMM":
                  return ye(Se, ue);
                case "D":
                  return H.$D;
                case "DD":
                  return A.s(H.$D, 2, "0");
                case "d":
                  return String(H.$W);
                case "dd":
                  return ye(F.weekdaysMin, H.$W, de, 2);
                case "ddd":
                  return ye(F.weekdaysShort, H.$W, de, 3);
                case "dddd":
                  return de[H.$W];
                case "H":
                  return String(Z);
                case "HH":
                  return A.s(Z, 2, "0");
                case "h":
                  return be(1);
                case "hh":
                  return be(2);
                case "a":
                  return we(Z, Q, !0);
                case "A":
                  return we(Z, Q, !1);
                case "m":
                  return String(Q);
                case "mm":
                  return A.s(Q, 2, "0");
                case "s":
                  return String(H.$s);
                case "ss":
                  return A.s(H.$s, 2, "0");
                case "SSS":
                  return A.s(H.$ms, 3, "0");
                case "Z":
                  return z;
              }
              return null;
            })(Ee) || z.replace(":", "");
          }));
        }, D.utcOffset = function() {
          return 15 * -Math.round(this.$d.getTimezoneOffset() / 15);
        }, D.diff = function($, H, F) {
          var q, z = this, Z = A.p(H), Q = P($), ue = (Q.utcOffset() - this.utcOffset()) * r, de = this - Q, Se = function() {
            return A.m(z, Q);
          };
          switch (Z) {
            case h:
              q = Se() / 12;
              break;
            case d:
              q = Se();
              break;
            case p:
              q = Se() / 3;
              break;
            case c:
              q = (de - ue) / 6048e5;
              break;
            case u:
              q = (de - ue) / 864e5;
              break;
            case l:
              q = de / n;
              break;
            case s:
              q = de / r;
              break;
            case i:
              q = de / a;
              break;
            default:
              q = de;
          }
          return F ? q : A.a(q);
        }, D.daysInMonth = function() {
          return this.endOf(d).$D;
        }, D.$locale = function() {
          return E[this.$L];
        }, D.locale = function($, H) {
          if (!$) return this.$L;
          var F = this.clone(), q = O($, H, !0);
          return q && (F.$L = q), F;
        }, D.clone = function() {
          return A.w(this.$d, this);
        }, D.toDate = function() {
          return new Date(this.valueOf());
        }, D.toJSON = function() {
          return this.isValid() ? this.toISOString() : null;
        }, D.toISOString = function() {
          return this.$d.toISOString();
        }, D.toString = function() {
          return this.$d.toUTCString();
        }, M;
      })(), C = v.prototype;
      return P.prototype = C, [["$ms", o], ["$s", i], ["$m", s], ["$H", l], ["$W", u], ["$M", d], ["$y", h], ["$D", g]].forEach((function(M) {
        C[M[1]] = function(D) {
          return this.$g(D, M[0], M[1]);
        };
      })), P.extend = function(M, D) {
        return M.$i || (M(D, v, P), M.$i = !0), P;
      }, P.locale = O, P.isDayjs = W, P.unix = function(M) {
        return P(1e3 * M);
      }, P.en = E[x], P.Ls = E, P.p = {}, P;
    }));
  })(_a)), _a.exports;
}
var mb = hb();
const ao = /* @__PURE__ */ Tr(mb);
var Pa = { exports: {} }, gb = Pa.exports, Kl;
function vb() {
  return Kl || (Kl = 1, (function(t, e) {
    (function(a, r) {
      t.exports = r();
    })(gb, (function() {
      var a = { LTS: "h:mm:ss A", LT: "h:mm A", L: "MM/DD/YYYY", LL: "MMMM D, YYYY", LLL: "MMMM D, YYYY h:mm A", LLLL: "dddd, MMMM D, YYYY h:mm A" }, r = /(\[[^[]*\])|([-_:/.,()\s]+)|(A|a|Q|YYYY|YY?|ww?|MM?M?M?|Do|DD?|hh?|HH?|mm?|ss?|S{1,3}|z|ZZ?)/g, n = /\d/, o = /\d\d/, i = /\d\d?/, s = /\d*[^-_:/,()\s\d]+/, l = {}, u = function(y) {
        return (y = +y) + (y > 68 ? 1900 : 2e3);
      }, c = function(y) {
        return function(w) {
          this[y] = +w;
        };
      }, d = [/[+-]\d\d:?(\d\d)?|Z/, function(y) {
        (this.zone || (this.zone = {})).offset = (function(w) {
          if (!w || w === "Z") return 0;
          var k = w.match(/([+-]|\d\d)/g), j = 60 * k[1] + (+k[2] || 0);
          return j === 0 ? 0 : k[0] === "+" ? -j : j;
        })(y);
      }], p = function(y) {
        var w = l[y];
        return w && (w.indexOf ? w : w.s.concat(w.f));
      }, h = function(y, w) {
        var k, j = l.meridiem;
        if (j) {
          for (var f = 1; f <= 24; f += 1) if (y.indexOf(j(f, 0, w)) > -1) {
            k = f > 12;
            break;
          }
        } else k = y === (w ? "pm" : "PM");
        return k;
      }, g = { A: [s, function(y) {
        this.afternoon = h(y, !1);
      }], a: [s, function(y) {
        this.afternoon = h(y, !0);
      }], Q: [n, function(y) {
        this.month = 3 * (y - 1) + 1;
      }], S: [n, function(y) {
        this.milliseconds = 100 * +y;
      }], SS: [o, function(y) {
        this.milliseconds = 10 * +y;
      }], SSS: [/\d{3}/, function(y) {
        this.milliseconds = +y;
      }], s: [i, c("seconds")], ss: [i, c("seconds")], m: [i, c("minutes")], mm: [i, c("minutes")], H: [i, c("hours")], h: [i, c("hours")], HH: [i, c("hours")], hh: [i, c("hours")], D: [i, c("day")], DD: [o, c("day")], Do: [s, function(y) {
        var w = l.ordinal, k = y.match(/\d+/);
        if (this.day = k[0], w) for (var j = 1; j <= 31; j += 1) w(j).replace(/\[|\]/g, "") === y && (this.day = j);
      }], w: [i, c("week")], ww: [o, c("week")], M: [i, c("month")], MM: [o, c("month")], MMM: [s, function(y) {
        var w = p("months"), k = (p("monthsShort") || w.map((function(j) {
          return j.slice(0, 3);
        }))).indexOf(y) + 1;
        if (k < 1) throw new Error();
        this.month = k % 12 || k;
      }], MMMM: [s, function(y) {
        var w = p("months").indexOf(y) + 1;
        if (w < 1) throw new Error();
        this.month = w % 12 || w;
      }], Y: [/[+-]?\d+/, c("year")], YY: [o, function(y) {
        this.year = u(y);
      }], YYYY: [/\d{4}/, c("year")], Z: d, ZZ: d };
      function m(y) {
        var w, k;
        w = y, k = l && l.formats;
        for (var j = (y = w.replace(/(\[[^\]]+])|(LTS?|l{1,4}|L{1,4})/g, (function(P, A, v) {
          var C = v && v.toUpperCase();
          return A || k[v] || a[v] || k[C].replace(/(\[[^\]]+])|(MMMM|MM|DD|dddd)/g, (function(M, D, $) {
            return D || $.slice(1);
          }));
        }))).match(r), f = j.length, x = 0; x < f; x += 1) {
          var E = j[x], L = g[E], W = L && L[0], O = L && L[1];
          j[x] = O ? { regex: W, parser: O } : E.replace(/^\[|\]$/g, "");
        }
        return function(P) {
          for (var A = {}, v = 0, C = 0; v < f; v += 1) {
            var M = j[v];
            if (typeof M == "string") C += M.length;
            else {
              var D = M.regex, $ = M.parser, H = P.slice(C), F = D.exec(H)[0];
              $.call(A, F), P = P.replace(F, "");
            }
          }
          return (function(q) {
            var z = q.afternoon;
            if (z !== void 0) {
              var Z = q.hours;
              z ? Z < 12 && (q.hours += 12) : Z === 12 && (q.hours = 0), delete q.afternoon;
            }
          })(A), A;
        };
      }
      return function(y, w, k) {
        k.p.customParseFormat = !0, y && y.parseTwoDigitYear && (u = y.parseTwoDigitYear);
        var j = w.prototype, f = j.parse;
        j.parse = function(x) {
          var E = x.date, L = x.utc, W = x.args;
          this.$u = L;
          var O = W[1];
          if (typeof O == "string") {
            var P = W[2] === !0, A = W[3] === !0, v = P || A, C = W[2];
            A && (C = W[2]), l = this.$locale(), !P && C && (l = k.Ls[C]), this.$d = (function(H, F, q, z) {
              try {
                if (["x", "X"].indexOf(F) > -1) return new Date((F === "X" ? 1e3 : 1) * H);
                var Z = m(F)(H), Q = Z.year, ue = Z.month, de = Z.day, Se = Z.hours, fe = Z.minutes, ye = Z.seconds, be = Z.milliseconds, we = Z.zone, Ee = Z.week, ge = /* @__PURE__ */ new Date(), ke = de || (Q || ue ? 1 : ge.getDate()), je = Q || ge.getFullYear(), Te = 0;
                Q && !ue || (Te = ue > 0 ? ue - 1 : ge.getMonth());
                var He, _e = Se || 0, Fe = fe || 0, We = ye || 0, Ue = be || 0;
                return we ? new Date(Date.UTC(je, Te, ke, _e, Fe, We, Ue + 60 * we.offset * 1e3)) : q ? new Date(Date.UTC(je, Te, ke, _e, Fe, We, Ue)) : (He = new Date(je, Te, ke, _e, Fe, We, Ue), Ee && (He = z(He).week(Ee).toDate()), He);
              } catch {
                return /* @__PURE__ */ new Date("");
              }
            })(E, O, L, k), this.init(), C && C !== !0 && (this.$L = this.locale(C).$L), v && E != this.format(O) && (this.$d = /* @__PURE__ */ new Date("")), l = {};
          } else if (O instanceof Array) for (var M = O.length, D = 1; D <= M; D += 1) {
            W[1] = O[D - 1];
            var $ = k.apply(this, W);
            if ($.isValid()) {
              this.$d = $.$d, this.$L = $.$L, this.init();
              break;
            }
            D === M && (this.$d = /* @__PURE__ */ new Date(""));
          }
          else f.call(this, x);
        };
      };
    }));
  })(Pa)), Pa.exports;
}
var bb = vb();
const yb = /* @__PURE__ */ Tr(bb);
var ja = { exports: {} }, xb = ja.exports, Ul;
function Cb() {
  return Ul || (Ul = 1, (function(t, e) {
    (function(a, r) {
      t.exports = r();
    })(xb, (function() {
      var a = { year: 0, month: 1, day: 2, hour: 3, minute: 4, second: 5 }, r = {};
      return function(n, o, i) {
        var s, l = function(p, h, g) {
          g === void 0 && (g = {});
          var m = new Date(p), y = (function(w, k) {
            k === void 0 && (k = {});
            var j = k.timeZoneName || "short", f = w + "|" + j, x = r[f];
            return x || (x = new Intl.DateTimeFormat("en-US", { hour12: !1, timeZone: w, year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", timeZoneName: j }), r[f] = x), x;
          })(h, g);
          return y.formatToParts(m);
        }, u = function(p, h) {
          for (var g = l(p, h), m = [], y = 0; y < g.length; y += 1) {
            var w = g[y], k = w.type, j = w.value, f = a[k];
            f >= 0 && (m[f] = parseInt(j, 10));
          }
          var x = m[3], E = x === 24 ? 0 : x, L = m[0] + "-" + m[1] + "-" + m[2] + " " + E + ":" + m[4] + ":" + m[5] + ":000", W = +p;
          return (i.utc(L).valueOf() - (W -= W % 1e3)) / 6e4;
        }, c = o.prototype;
        c.tz = function(p, h) {
          p === void 0 && (p = s);
          var g, m = this.utcOffset(), y = this.toDate(), w = y.toLocaleString("en-US", { timeZone: p }), k = Math.round((y - new Date(w)) / 1e3 / 60), j = 15 * -Math.round(y.getTimezoneOffset() / 15) - k;
          if (!Number(j)) g = this.utcOffset(0, h);
          else if (g = i(w, { locale: this.$L }).$set("millisecond", this.$ms).utcOffset(j, !0), h) {
            var f = g.utcOffset();
            g = g.add(m - f, "minute");
          }
          return g.$x.$timezone = p, g;
        }, c.offsetName = function(p) {
          var h = this.$x.$timezone || i.tz.guess(), g = l(this.valueOf(), h, { timeZoneName: p }).find((function(m) {
            return m.type.toLowerCase() === "timezonename";
          }));
          return g && g.value;
        };
        var d = c.startOf;
        c.startOf = function(p, h) {
          if (!this.$x || !this.$x.$timezone) return d.call(this, p, h);
          var g = i(this.format("YYYY-MM-DD HH:mm:ss:SSS"), { locale: this.$L });
          return d.call(g, p, h).tz(this.$x.$timezone, !0);
        }, i.tz = function(p, h, g) {
          var m = g && h, y = g || h || s, w = u(+i(), y);
          if (typeof p != "string") return i(p).tz(y);
          var k = (function(E, L, W) {
            var O = E - 60 * L * 1e3, P = u(O, W);
            if (L === P) return [O, L];
            var A = u(O -= 60 * (P - L) * 1e3, W);
            return P === A ? [O, P] : [E - 60 * Math.min(P, A) * 1e3, Math.max(P, A)];
          })(i.utc(p, m).valueOf(), w, y), j = k[0], f = k[1], x = i(j).utcOffset(f);
          return x.$x.$timezone = y, x;
        }, i.tz.guess = function() {
          return Intl.DateTimeFormat().resolvedOptions().timeZone;
        }, i.tz.setDefault = function(p) {
          s = p;
        };
      };
    }));
  })(ja)), ja.exports;
}
var Sb = Cb();
const wb = /* @__PURE__ */ Tr(Sb);
var Ia = { exports: {} }, Eb = Ia.exports, ql;
function kb() {
  return ql || (ql = 1, (function(t, e) {
    (function(a, r) {
      t.exports = r();
    })(Eb, (function() {
      var a = "minute", r = /[+-]\d\d(?::?\d\d)?/g, n = /([+-]|\d\d)/g;
      return function(o, i, s) {
        var l = i.prototype;
        s.utc = function(m) {
          var y = { date: m, utc: !0, args: arguments };
          return new i(y);
        }, l.utc = function(m) {
          var y = s(this.toDate(), { locale: this.$L, utc: !0 });
          return m ? y.add(this.utcOffset(), a) : y;
        }, l.local = function() {
          return s(this.toDate(), { locale: this.$L, utc: !1 });
        };
        var u = l.parse;
        l.parse = function(m) {
          m.utc && (this.$u = !0), this.$utils().u(m.$offset) || (this.$offset = m.$offset), u.call(this, m);
        };
        var c = l.init;
        l.init = function() {
          if (this.$u) {
            var m = this.$d;
            this.$y = m.getUTCFullYear(), this.$M = m.getUTCMonth(), this.$D = m.getUTCDate(), this.$W = m.getUTCDay(), this.$H = m.getUTCHours(), this.$m = m.getUTCMinutes(), this.$s = m.getUTCSeconds(), this.$ms = m.getUTCMilliseconds();
          } else c.call(this);
        };
        var d = l.utcOffset;
        l.utcOffset = function(m, y) {
          var w = this.$utils().u;
          if (w(m)) return this.$u ? 0 : w(this.$offset) ? d.call(this) : this.$offset;
          if (typeof m == "string" && (m = (function(x) {
            x === void 0 && (x = "");
            var E = x.match(r);
            if (!E) return null;
            var L = ("" + E[0]).match(n) || ["-", 0, 0], W = L[0], O = 60 * +L[1] + +L[2];
            return O === 0 ? 0 : W === "+" ? O : -O;
          })(m), m === null)) return this;
          var k = Math.abs(m) <= 16 ? 60 * m : m;
          if (k === 0) return this.utc(y);
          var j = this.clone();
          if (y) return j.$offset = k, j.$u = !1, j;
          var f = this.$u ? this.toDate().getTimezoneOffset() : -1 * this.utcOffset();
          return (j = this.local().add(k + f, a)).$offset = k, j.$x.$localOffset = f, j;
        };
        var p = l.format;
        l.format = function(m) {
          var y = m || (this.$u ? "YYYY-MM-DDTHH:mm:ss[Z]" : "");
          return p.call(this, y);
        }, l.valueOf = function() {
          var m = this.$utils().u(this.$offset) ? 0 : this.$offset + (this.$x.$localOffset || this.$d.getTimezoneOffset());
          return this.$d.valueOf() - 6e4 * m;
        }, l.isUTC = function() {
          return !!this.$u;
        }, l.toISOString = function() {
          return this.toDate().toISOString();
        }, l.toString = function() {
          return this.toDate().toUTCString();
        };
        var h = l.toDate;
        l.toDate = function(m) {
          return m === "s" && this.$offset ? s(this.format("YYYY-MM-DD HH:mm:ss:SSS")).toDate() : h.call(this);
        };
        var g = l.diff;
        l.diff = function(m, y, w) {
          if (m && this.$u === m.$u) return g.call(this, m, y, w);
          var k = this.local(), j = s(m).local();
          return g.call(k, j, y, w);
        };
      };
    }));
  })(Ia)), Ia.exports;
}
var $b = kb();
const Rb = /* @__PURE__ */ Tr($b);
ao.extend(wb);
ao.extend(Rb);
ao.extend(yb);
function Tb(t, e, a = "HH:mm:ss") {
  return ao(t).tz(e).format(a);
}
var $c;
const Ob = (($c = Intl.DateTimeFormat().resolvedOptions().timeZone) == null ? void 0 : $c.toString()) || "UTC", _b = (t) => typeof t == "string" ? t === "" ? /* @__PURE__ */ new Date() : new Date(t) : t, Pb = (t, e = Ob) => {
  const a = _b(t);
  return `${Tb(
    a,
    e,
    "YYYY-MM-DDTHH:mm:ss.SSSZ"
  )}`;
}, jb = "INFO", Ib = "INFO", Ab = "WARN", Mb = "ERROR", Nb = "FAILED", Lb = "SUCCEEDED", Lu = ar(
  (t) => fr({
    disableUserSelect: {
      userSelect: "none"
    },
    "@global": {
      "@keyframes refreshRotation": {
        from: {
          transform: "rotate(0deg)"
        },
        to: {
          transform: "rotate(360deg)"
        }
      }
    },
    highlightedLogLine: {
      backgroundColor: "#D8E1E8"
    },
    selectedLogLine: {
      backgroundColor: Ye("#ccd1f2", 1)
    },
    traceIdAvailableLogLine: {
      backgroundColor: "#E5E4E2",
      cursor: "point"
    },
    whiteBackground: {
      backgroundColor: t.palette.common.white
    },
    logLineWrapper: { display: "flex" },
    expandedLogKey: {
      width: t.spacing(35),
      paddingLeft: t.spacing(0.5)
    },
    defaultLogLine: {
      fontFamily: "Droid Sans Mono",
      minWidth: "100%",
      paddingLeft: t.spacing(0.4),
      fontSize: t.spacing(1.375),
      transitionDuration: "0.25s",
      transitionProperty: "background-color",
      overflow: "visible",
      minHeight: t.spacing(3),
      display: "inline-flex",
      diplayDirection: "row",
      "& $actionContainer": {
        visibility: "hidden"
      },
      "&:hover": {
        background: Ye("#E5E4E2", 1)
      }
    },
    "@keyframes animFade": {
      from: {
        opacity: 0
      },
      to: { opacity: 1 }
    },
    toolTipParent: {
      display: "inline-flex",
      lineHeight: t.spacing(0.25),
      "& $toolTipChild": {
        visibility: "hidden"
      },
      "&:hover": {
        "& $toolTipChild": {
          visibility: "visible",
          animation: "$animFade 250ms 1"
        }
      }
    },
    toolTipChild: {
      top: t.spacing(-4),
      padding: t.spacing(0.5),
      color: t.palette.primary.main,
      display: "flex",
      background: t.palette.common.white,
      boxShadow: "0 1px 3px rgba(0, 0, 0, 0.19)",
      userSelect: "none",
      borderRadius: t.spacing(0.625),
      position: "absolute",
      cursor: "pointer",
      "&:hover": {
        backgroundColor: t.palette.common.white
      }
    },
    logEntryActions: {
      position: "relative",
      display: "flex",
      paddingRight: t.spacing(1.25)
    },
    actionContainer: {
      top: t.spacing(-6),
      padding: t.spacing(0.5),
      display: "flex",
      width: "max-content",
      background: t.palette.common.white,
      position: "absolute",
      cursor: "pointer",
      border: `1px solid ${t.palette.grey[100]}`,
      borderColor: t.palette.grey[100],
      borderRadius: t.spacing(0.625),
      backgroundColor: t.palette.common.white,
      boxShadow: "0 1px 3px rgba(0, 0, 0, 0.19)"
    },
    actionButton: {
      display: "flex",
      alignItems: "center",
      padding: `${t.spacing(0.5)}px ${t.spacing(1)}px`,
      gap: t.spacing(0.5),
      height: t.spacing(3),
      backgroundColor: Ye(t.palette.common.white, 0),
      flex: "none",
      border: t.palette.grey[100],
      cursor: "pointer",
      "&:hover": {
        "& $actionIcon": {
          color: t.palette.primary.light
        },
        "& $actionLabel": {
          color: t.palette.primary.light
        }
      }
    },
    actionIcon: {
      width: t.spacing(1.625),
      height: t.spacing(1.625),
      color: t.palette.primary.main
    },
    actionLabel: {
      fontFamily: t.typography.fontFamily,
      fontWeight: 400,
      fontSize: t.spacing(1.625),
      lineHeight: t.spacing(0.23),
      color: t.palette.primary.main
    },
    errorLogs: {
      color: "#EA4C4D",
      lineHeight: t.spacing(0.25)
    },
    infoLogs: {
      color: "#53C08A",
      lineHeight: t.spacing(0.25)
    },
    warningLogs: {
      color: "#FF9D52",
      lineHeight: t.spacing(0.25)
    },
    logOther: {
      color: t.palette.secondary.dark,
      lineHeight: t.spacing(0.25)
    },
    logContextKeyValue: {
      color: "#0095ff"
    },
    logMsg: {
      lineHeight: t.spacing(0.25),
      display: "inline",
      overflow: "visible",
      whiteSpace: "nowrap",
      "&:hover": {
        "& $actionContainer": {
          visibility: "visible",
          animation: "$animFade 250ms 1",
          backgroundColor: t.palette.common.white
        }
      }
    },
    infoSection: {
      display: "inline",
      whiteSpace: "nowrap"
    },
    arrow: {
      color: t.palette.primary.main,
      fontSize: t.spacing(1),
      lineHeight: t.spacing(0.25),
      padding: t.spacing(0.5),
      cursor: "pointer"
    },
    logContextDivider: {
      color: t.palette.divider
    },
    lastLogTimeStamp: {
      borderBottom: `1px dashed ${t.palette.primary.main}`
    },
    // Note: Added important to avoid overriding by virtualize lib
    // Note: Added type casting to 'auto !important' as 'auto' to avoid type error
    logsList: {
      overflowX: "auto !important"
    },
    expandedLogRow: {
      width: "fit-content",
      borderBottom: "1px solid #E5E4E2",
      padding: t.spacing(0.5, 0),
      display: "flex",
      backgroundColor: t.palette.common.white
    },
    tableContainer: {
      display: "flex",
      flexDirection: "column",
      paddingLeft: t.spacing(8),
      overflowX: "visible",
      whiteSpace: "nowrap"
    },
    logsListContainer: {
      width: "100%",
      height: "100%",
      paddingLeft: t.spacing(2),
      backgroundColor: t.palette.common.white
    },
    logColor1: {
      color: "#745BCC"
    },
    logColor2: {
      color: "#2366CC"
    },
    logColor3: {
      color: "#2366CC"
    },
    logColor4: {
      color: "#0095ff"
    },
    alertMetric: {
      filter: "hue-rotate(45deg)"
    }
  })
);
var Aa = { exports: {} }, Db = Aa.exports, Gl;
function Bb() {
  return Gl || (Gl = 1, (function(t, e) {
    (function(a, r) {
      t.exports = r(ht);
    })(Db, (function(a) {
      return (function(r) {
        var n = {};
        function o(i) {
          if (n[i]) return n[i].exports;
          var s = n[i] = { i, l: !1, exports: {} };
          return r[i].call(s.exports, s, s.exports, o), s.l = !0, s.exports;
        }
        return o.m = r, o.c = n, o.d = function(i, s, l) {
          o.o(i, s) || Object.defineProperty(i, s, { enumerable: !0, get: l });
        }, o.r = function(i) {
          typeof Symbol < "u" && Symbol.toStringTag && Object.defineProperty(i, Symbol.toStringTag, { value: "Module" }), Object.defineProperty(i, "__esModule", { value: !0 });
        }, o.t = function(i, s) {
          if (1 & s && (i = o(i)), 8 & s || 4 & s && typeof i == "object" && i && i.__esModule) return i;
          var l = /* @__PURE__ */ Object.create(null);
          if (o.r(l), Object.defineProperty(l, "default", { enumerable: !0, value: i }), 2 & s && typeof i != "string") for (var u in i) o.d(l, u, (function(c) {
            return i[c];
          }).bind(null, u));
          return l;
        }, o.n = function(i) {
          var s = i && i.__esModule ? function() {
            return i.default;
          } : function() {
            return i;
          };
          return o.d(s, "a", s), s;
        }, o.o = function(i, s) {
          return Object.prototype.hasOwnProperty.call(i, s);
        }, o.p = "", o(o.s = 48);
      })([function(r, n) {
        r.exports = a;
      }, function(r, n) {
        var o = r.exports = { version: "2.6.12" };
        typeof __e == "number" && (__e = o);
      }, function(r, n, o) {
        var i = o(26)("wks"), s = o(17), l = o(3).Symbol, u = typeof l == "function";
        (r.exports = function(c) {
          return i[c] || (i[c] = u && l[c] || (u ? l : s)("Symbol." + c));
        }).store = i;
      }, function(r, n) {
        var o = r.exports = typeof window < "u" && window.Math == Math ? window : typeof self < "u" && self.Math == Math ? self : Function("return this")();
        typeof __g == "number" && (__g = o);
      }, function(r, n, o) {
        r.exports = !o(8)((function() {
          return Object.defineProperty({}, "a", { get: function() {
            return 7;
          } }).a != 7;
        }));
      }, function(r, n) {
        var o = {}.hasOwnProperty;
        r.exports = function(i, s) {
          return o.call(i, s);
        };
      }, function(r, n, o) {
        var i = o(7), s = o(16);
        r.exports = o(4) ? function(l, u, c) {
          return i.f(l, u, s(1, c));
        } : function(l, u, c) {
          return l[u] = c, l;
        };
      }, function(r, n, o) {
        var i = o(10), s = o(35), l = o(23), u = Object.defineProperty;
        n.f = o(4) ? Object.defineProperty : function(c, d, p) {
          if (i(c), d = l(d, !0), i(p), s) try {
            return u(c, d, p);
          } catch {
          }
          if ("get" in p || "set" in p) throw TypeError("Accessors not supported!");
          return "value" in p && (c[d] = p.value), c;
        };
      }, function(r, n) {
        r.exports = function(o) {
          try {
            return !!o();
          } catch {
            return !0;
          }
        };
      }, function(r, n, o) {
        var i = o(40), s = o(22);
        r.exports = function(l) {
          return i(s(l));
        };
      }, function(r, n, o) {
        var i = o(11);
        r.exports = function(s) {
          if (!i(s)) throw TypeError(s + " is not an object!");
          return s;
        };
      }, function(r, n) {
        r.exports = function(o) {
          return typeof o == "object" ? o !== null : typeof o == "function";
        };
      }, function(r, n) {
        r.exports = {};
      }, function(r, n, o) {
        var i = o(39), s = o(27);
        r.exports = Object.keys || function(l) {
          return i(l, s);
        };
      }, function(r, n) {
        r.exports = !0;
      }, function(r, n, o) {
        var i = o(3), s = o(1), l = o(53), u = o(6), c = o(5), d = function(p, h, g) {
          var m, y, w, k = p & d.F, j = p & d.G, f = p & d.S, x = p & d.P, E = p & d.B, L = p & d.W, W = j ? s : s[h] || (s[h] = {}), O = W.prototype, P = j ? i : f ? i[h] : (i[h] || {}).prototype;
          for (m in j && (g = h), g) (y = !k && P && P[m] !== void 0) && c(W, m) || (w = y ? P[m] : g[m], W[m] = j && typeof P[m] != "function" ? g[m] : E && y ? l(w, i) : L && P[m] == w ? (function(A) {
            var v = function(C, M, D) {
              if (this instanceof A) {
                switch (arguments.length) {
                  case 0:
                    return new A();
                  case 1:
                    return new A(C);
                  case 2:
                    return new A(C, M);
                }
                return new A(C, M, D);
              }
              return A.apply(this, arguments);
            };
            return v.prototype = A.prototype, v;
          })(w) : x && typeof w == "function" ? l(Function.call, w) : w, x && ((W.virtual || (W.virtual = {}))[m] = w, p & d.R && O && !O[m] && u(O, m, w)));
        };
        d.F = 1, d.G = 2, d.S = 4, d.P = 8, d.B = 16, d.W = 32, d.U = 64, d.R = 128, r.exports = d;
      }, function(r, n) {
        r.exports = function(o, i) {
          return { enumerable: !(1 & o), configurable: !(2 & o), writable: !(4 & o), value: i };
        };
      }, function(r, n) {
        var o = 0, i = Math.random();
        r.exports = function(s) {
          return "Symbol(".concat(s === void 0 ? "" : s, ")_", (++o + i).toString(36));
        };
      }, function(r, n, o) {
        var i = o(22);
        r.exports = function(s) {
          return Object(i(s));
        };
      }, function(r, n) {
        n.f = {}.propertyIsEnumerable;
      }, function(r, n, o) {
        var i = o(52)(!0);
        o(34)(String, "String", (function(s) {
          this._t = String(s), this._i = 0;
        }), (function() {
          var s, l = this._t, u = this._i;
          return u >= l.length ? { value: void 0, done: !0 } : (s = i(l, u), this._i += s.length, { value: s, done: !1 });
        }));
      }, function(r, n) {
        var o = Math.ceil, i = Math.floor;
        r.exports = function(s) {
          return isNaN(s = +s) ? 0 : (s > 0 ? i : o)(s);
        };
      }, function(r, n) {
        r.exports = function(o) {
          if (o == null) throw TypeError("Can't call method on  " + o);
          return o;
        };
      }, function(r, n, o) {
        var i = o(11);
        r.exports = function(s, l) {
          if (!i(s)) return s;
          var u, c;
          if (l && typeof (u = s.toString) == "function" && !i(c = u.call(s)) || typeof (u = s.valueOf) == "function" && !i(c = u.call(s)) || !l && typeof (u = s.toString) == "function" && !i(c = u.call(s))) return c;
          throw TypeError("Can't convert object to primitive value");
        };
      }, function(r, n) {
        var o = {}.toString;
        r.exports = function(i) {
          return o.call(i).slice(8, -1);
        };
      }, function(r, n, o) {
        var i = o(26)("keys"), s = o(17);
        r.exports = function(l) {
          return i[l] || (i[l] = s(l));
        };
      }, function(r, n, o) {
        var i = o(1), s = o(3), l = s["__core-js_shared__"] || (s["__core-js_shared__"] = {});
        (r.exports = function(u, c) {
          return l[u] || (l[u] = c !== void 0 ? c : {});
        })("versions", []).push({ version: i.version, mode: o(14) ? "pure" : "global", copyright: "© 2020 Denis Pushkarev (zloirock.ru)" });
      }, function(r, n) {
        r.exports = "constructor,hasOwnProperty,isPrototypeOf,propertyIsEnumerable,toLocaleString,toString,valueOf".split(",");
      }, function(r, n, o) {
        var i = o(7).f, s = o(5), l = o(2)("toStringTag");
        r.exports = function(u, c, d) {
          u && !s(u = d ? u : u.prototype, l) && i(u, l, { configurable: !0, value: c });
        };
      }, function(r, n, o) {
        o(62);
        for (var i = o(3), s = o(6), l = o(12), u = o(2)("toStringTag"), c = "CSSRuleList,CSSStyleDeclaration,CSSValueList,ClientRectList,DOMRectList,DOMStringList,DOMTokenList,DataTransferItemList,FileList,HTMLAllCollection,HTMLCollection,HTMLFormElement,HTMLSelectElement,MediaList,MimeTypeArray,NamedNodeMap,NodeList,PaintRequestList,Plugin,PluginArray,SVGLengthList,SVGNumberList,SVGPathSegList,SVGPointList,SVGStringList,SVGTransformList,SourceBufferList,StyleSheetList,TextTrackCueList,TextTrackList,TouchList".split(","), d = 0; d < c.length; d++) {
          var p = c[d], h = i[p], g = h && h.prototype;
          g && !g[u] && s(g, u, p), l[p] = l.Array;
        }
      }, function(r, n, o) {
        n.f = o(2);
      }, function(r, n, o) {
        var i = o(3), s = o(1), l = o(14), u = o(30), c = o(7).f;
        r.exports = function(d) {
          var p = s.Symbol || (s.Symbol = l ? {} : i.Symbol || {});
          d.charAt(0) == "_" || d in p || c(p, d, { value: u.f(d) });
        };
      }, function(r, n) {
        n.f = Object.getOwnPropertySymbols;
      }, function(r, n) {
        r.exports = function(o, i, s) {
          return Math.min(Math.max(o, i), s);
        };
      }, function(r, n, o) {
        var i = o(14), s = o(15), l = o(37), u = o(6), c = o(12), d = o(55), p = o(28), h = o(61), g = o(2)("iterator"), m = !([].keys && "next" in [].keys()), y = function() {
          return this;
        };
        r.exports = function(w, k, j, f, x, E, L) {
          d(j, k, f);
          var W, O, P, A = function(z) {
            if (!m && z in D) return D[z];
            switch (z) {
              case "keys":
              case "values":
                return function() {
                  return new j(this, z);
                };
            }
            return function() {
              return new j(this, z);
            };
          }, v = k + " Iterator", C = x == "values", M = !1, D = w.prototype, $ = D[g] || D["@@iterator"] || x && D[x], H = $ || A(x), F = x ? C ? A("entries") : H : void 0, q = k == "Array" && D.entries || $;
          if (q && (P = h(q.call(new w()))) !== Object.prototype && P.next && (p(P, v, !0), i || typeof P[g] == "function" || u(P, g, y)), C && $ && $.name !== "values" && (M = !0, H = function() {
            return $.call(this);
          }), i && !L || !m && !M && D[g] || u(D, g, H), c[k] = H, c[v] = y, x) if (W = { values: C ? H : A("values"), keys: E ? H : A("keys"), entries: F }, L) for (O in W) O in D || l(D, O, W[O]);
          else s(s.P + s.F * (m || M), k, W);
          return W;
        };
      }, function(r, n, o) {
        r.exports = !o(4) && !o(8)((function() {
          return Object.defineProperty(o(36)("div"), "a", { get: function() {
            return 7;
          } }).a != 7;
        }));
      }, function(r, n, o) {
        var i = o(11), s = o(3).document, l = i(s) && i(s.createElement);
        r.exports = function(u) {
          return l ? s.createElement(u) : {};
        };
      }, function(r, n, o) {
        r.exports = o(6);
      }, function(r, n, o) {
        var i = o(10), s = o(56), l = o(27), u = o(25)("IE_PROTO"), c = function() {
        }, d = function() {
          var p, h = o(36)("iframe"), g = l.length;
          for (h.style.display = "none", o(60).appendChild(h), h.src = "javascript:", (p = h.contentWindow.document).open(), p.write("<script>document.F=Object<\/script>"), p.close(), d = p.F; g--; ) delete d.prototype[l[g]];
          return d();
        };
        r.exports = Object.create || function(p, h) {
          var g;
          return p !== null ? (c.prototype = i(p), g = new c(), c.prototype = null, g[u] = p) : g = d(), h === void 0 ? g : s(g, h);
        };
      }, function(r, n, o) {
        var i = o(5), s = o(9), l = o(57)(!1), u = o(25)("IE_PROTO");
        r.exports = function(c, d) {
          var p, h = s(c), g = 0, m = [];
          for (p in h) p != u && i(h, p) && m.push(p);
          for (; d.length > g; ) i(h, p = d[g++]) && (~l(m, p) || m.push(p));
          return m;
        };
      }, function(r, n, o) {
        var i = o(24);
        r.exports = Object("z").propertyIsEnumerable(0) ? Object : function(s) {
          return i(s) == "String" ? s.split("") : Object(s);
        };
      }, function(r, n, o) {
        var i = o(39), s = o(27).concat("length", "prototype");
        n.f = Object.getOwnPropertyNames || function(l) {
          return i(l, s);
        };
      }, function(r, n, o) {
        var i = o(24), s = o(2)("toStringTag"), l = i(/* @__PURE__ */ (function() {
          return arguments;
        })()) == "Arguments";
        r.exports = function(u) {
          var c, d, p;
          return u === void 0 ? "Undefined" : u === null ? "Null" : typeof (d = (function(h, g) {
            try {
              return h[g];
            } catch {
            }
          })(c = Object(u), s)) == "string" ? d : l ? i(c) : (p = i(c)) == "Object" && typeof c.callee == "function" ? "Arguments" : p;
        };
      }, function(r, n) {
        var o;
        o = /* @__PURE__ */ (function() {
          return this;
        })();
        try {
          o = o || new Function("return this")();
        } catch {
          typeof window == "object" && (o = window);
        }
        r.exports = o;
      }, function(r, n) {
        var o = /-?\d+(\.\d+)?%?/g;
        r.exports = function(i) {
          return i.match(o);
        };
      }, function(r, n, o) {
        Object.defineProperty(n, "__esModule", { value: !0 }), n.getBase16Theme = n.createStyling = n.invertTheme = void 0;
        var i = y(o(49)), s = y(o(76)), l = y(o(81)), u = y(o(89)), c = y(o(93)), d = (function(O) {
          if (O && O.__esModule) return O;
          var P = {};
          if (O != null) for (var A in O) Object.prototype.hasOwnProperty.call(O, A) && (P[A] = O[A]);
          return P.default = O, P;
        })(o(94)), p = y(o(132)), h = y(o(133)), g = y(o(138)), m = o(139);
        function y(O) {
          return O && O.__esModule ? O : { default: O };
        }
        var w = d.default, k = (0, u.default)(w), j = (0, g.default)(h.default, m.rgb2yuv, (function(O) {
          var P, A = (0, l.default)(O, 3), v = A[0], C = A[1], M = A[2];
          return [(P = v, P < 0.25 ? 1 : P < 0.5 ? 0.9 - P : 1.1 - P), C, M];
        }), m.yuv2rgb, p.default), f = function(O) {
          return function(P) {
            return { className: [P.className, O.className].filter(Boolean).join(" "), style: (0, s.default)({}, P.style || {}, O.style || {}) };
          };
        }, x = function(O, P) {
          var A = (0, u.default)(P);
          for (var v in O) A.indexOf(v) === -1 && A.push(v);
          return A.reduce((function(C, M) {
            return C[M] = (function(D, $) {
              if (D === void 0) return $;
              if ($ === void 0) return D;
              var H = D === void 0 ? "undefined" : (0, i.default)(D), F = $ === void 0 ? "undefined" : (0, i.default)($);
              switch (H) {
                case "string":
                  switch (F) {
                    case "string":
                      return [$, D].filter(Boolean).join(" ");
                    case "object":
                      return f({ className: D, style: $ });
                    case "function":
                      return function(q) {
                        for (var z = arguments.length, Z = Array(z > 1 ? z - 1 : 0), Q = 1; Q < z; Q++) Z[Q - 1] = arguments[Q];
                        return f({ className: D })($.apply(void 0, [q].concat(Z)));
                      };
                  }
                case "object":
                  switch (F) {
                    case "string":
                      return f({ className: $, style: D });
                    case "object":
                      return (0, s.default)({}, $, D);
                    case "function":
                      return function(q) {
                        for (var z = arguments.length, Z = Array(z > 1 ? z - 1 : 0), Q = 1; Q < z; Q++) Z[Q - 1] = arguments[Q];
                        return f({ style: D })($.apply(void 0, [q].concat(Z)));
                      };
                  }
                case "function":
                  switch (F) {
                    case "string":
                      return function(q) {
                        for (var z = arguments.length, Z = Array(z > 1 ? z - 1 : 0), Q = 1; Q < z; Q++) Z[Q - 1] = arguments[Q];
                        return D.apply(void 0, [f(q)({ className: $ })].concat(Z));
                      };
                    case "object":
                      return function(q) {
                        for (var z = arguments.length, Z = Array(z > 1 ? z - 1 : 0), Q = 1; Q < z; Q++) Z[Q - 1] = arguments[Q];
                        return D.apply(void 0, [f(q)({ style: $ })].concat(Z));
                      };
                    case "function":
                      return function(q) {
                        for (var z = arguments.length, Z = Array(z > 1 ? z - 1 : 0), Q = 1; Q < z; Q++) Z[Q - 1] = arguments[Q];
                        return D.apply(void 0, [$.apply(void 0, [q].concat(Z))].concat(Z));
                      };
                  }
              }
            })(O[M], P[M]), C;
          }), {});
        }, E = function(O, P) {
          for (var A = arguments.length, v = Array(A > 2 ? A - 2 : 0), C = 2; C < A; C++) v[C - 2] = arguments[C];
          if (P === null) return O;
          Array.isArray(P) || (P = [P]);
          var M = P.map((function($) {
            return O[$];
          })).filter(Boolean), D = M.reduce((function($, H) {
            return typeof H == "string" ? $.className = [$.className, H].filter(Boolean).join(" ") : (H === void 0 ? "undefined" : (0, i.default)(H)) === "object" ? $.style = (0, s.default)({}, $.style, H) : typeof H == "function" && ($ = (0, s.default)({}, $, H.apply(void 0, [$].concat(v)))), $;
          }), { className: "", style: {} });
          return D.className || delete D.className, (0, u.default)(D.style).length === 0 && delete D.style, D;
        }, L = n.invertTheme = function(O) {
          return (0, u.default)(O).reduce((function(P, A) {
            return P[A] = /^base/.test(A) ? j(O[A]) : A === "scheme" ? O[A] + ":inverted" : O[A], P;
          }), {});
        }, W = (n.createStyling = (0, c.default)((function(O) {
          for (var P = arguments.length, A = Array(P > 3 ? P - 3 : 0), v = 3; v < P; v++) A[v - 3] = arguments[v];
          var C = arguments.length > 1 && arguments[1] !== void 0 ? arguments[1] : {}, M = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : {}, D = C.defaultBase16, $ = D === void 0 ? w : D, H = C.base16Themes, F = H === void 0 ? null : H, q = W(M, F);
          q && (M = (0, s.default)({}, q, M));
          var z = k.reduce((function(de, Se) {
            return de[Se] = M[Se] || $[Se], de;
          }), {}), Z = (0, u.default)(M).reduce((function(de, Se) {
            return k.indexOf(Se) === -1 && (de[Se] = M[Se]), de;
          }), {}), Q = O(z), ue = x(Z, Q);
          return (0, c.default)(E, 2).apply(void 0, [ue].concat(A));
        }), 3), n.getBase16Theme = function(O, P) {
          if (O && O.extend && (O = O.extend), typeof O == "string") {
            var A = O.split(":"), v = (0, l.default)(A, 2), C = v[0], M = v[1];
            O = (P || {})[C] || d[C], M === "inverted" && (O = L(O));
          }
          return O && O.hasOwnProperty("base00") ? O : void 0;
        });
      }, function(r, n, o) {
        var i, s = typeof Reflect == "object" ? Reflect : null, l = s && typeof s.apply == "function" ? s.apply : function(f, x, E) {
          return Function.prototype.apply.call(f, x, E);
        };
        i = s && typeof s.ownKeys == "function" ? s.ownKeys : Object.getOwnPropertySymbols ? function(f) {
          return Object.getOwnPropertyNames(f).concat(Object.getOwnPropertySymbols(f));
        } : function(f) {
          return Object.getOwnPropertyNames(f);
        };
        var u = Number.isNaN || function(f) {
          return f != f;
        };
        function c() {
          c.init.call(this);
        }
        r.exports = c, r.exports.once = function(f, x) {
          return new Promise((function(E, L) {
            function W() {
              O !== void 0 && f.removeListener("error", O), E([].slice.call(arguments));
            }
            var O;
            x !== "error" && (O = function(P) {
              f.removeListener(x, W), L(P);
            }, f.once("error", O)), f.once(x, W);
          }));
        }, c.EventEmitter = c, c.prototype._events = void 0, c.prototype._eventsCount = 0, c.prototype._maxListeners = void 0;
        var d = 10;
        function p(f) {
          if (typeof f != "function") throw new TypeError('The "listener" argument must be of type Function. Received type ' + typeof f);
        }
        function h(f) {
          return f._maxListeners === void 0 ? c.defaultMaxListeners : f._maxListeners;
        }
        function g(f, x, E, L) {
          var W, O, P, A;
          if (p(E), (O = f._events) === void 0 ? (O = f._events = /* @__PURE__ */ Object.create(null), f._eventsCount = 0) : (O.newListener !== void 0 && (f.emit("newListener", x, E.listener ? E.listener : E), O = f._events), P = O[x]), P === void 0) P = O[x] = E, ++f._eventsCount;
          else if (typeof P == "function" ? P = O[x] = L ? [E, P] : [P, E] : L ? P.unshift(E) : P.push(E), (W = h(f)) > 0 && P.length > W && !P.warned) {
            P.warned = !0;
            var v = new Error("Possible EventEmitter memory leak detected. " + P.length + " " + String(x) + " listeners added. Use emitter.setMaxListeners() to increase limit");
            v.name = "MaxListenersExceededWarning", v.emitter = f, v.type = x, v.count = P.length, A = v, console && console.warn && console.warn(A);
          }
          return f;
        }
        function m() {
          if (!this.fired) return this.target.removeListener(this.type, this.wrapFn), this.fired = !0, arguments.length === 0 ? this.listener.call(this.target) : this.listener.apply(this.target, arguments);
        }
        function y(f, x, E) {
          var L = { fired: !1, wrapFn: void 0, target: f, type: x, listener: E }, W = m.bind(L);
          return W.listener = E, L.wrapFn = W, W;
        }
        function w(f, x, E) {
          var L = f._events;
          if (L === void 0) return [];
          var W = L[x];
          return W === void 0 ? [] : typeof W == "function" ? E ? [W.listener || W] : [W] : E ? (function(O) {
            for (var P = new Array(O.length), A = 0; A < P.length; ++A) P[A] = O[A].listener || O[A];
            return P;
          })(W) : j(W, W.length);
        }
        function k(f) {
          var x = this._events;
          if (x !== void 0) {
            var E = x[f];
            if (typeof E == "function") return 1;
            if (E !== void 0) return E.length;
          }
          return 0;
        }
        function j(f, x) {
          for (var E = new Array(x), L = 0; L < x; ++L) E[L] = f[L];
          return E;
        }
        Object.defineProperty(c, "defaultMaxListeners", { enumerable: !0, get: function() {
          return d;
        }, set: function(f) {
          if (typeof f != "number" || f < 0 || u(f)) throw new RangeError('The value of "defaultMaxListeners" is out of range. It must be a non-negative number. Received ' + f + ".");
          d = f;
        } }), c.init = function() {
          this._events !== void 0 && this._events !== Object.getPrototypeOf(this)._events || (this._events = /* @__PURE__ */ Object.create(null), this._eventsCount = 0), this._maxListeners = this._maxListeners || void 0;
        }, c.prototype.setMaxListeners = function(f) {
          if (typeof f != "number" || f < 0 || u(f)) throw new RangeError('The value of "n" is out of range. It must be a non-negative number. Received ' + f + ".");
          return this._maxListeners = f, this;
        }, c.prototype.getMaxListeners = function() {
          return h(this);
        }, c.prototype.emit = function(f) {
          for (var x = [], E = 1; E < arguments.length; E++) x.push(arguments[E]);
          var L = f === "error", W = this._events;
          if (W !== void 0) L = L && W.error === void 0;
          else if (!L) return !1;
          if (L) {
            var O;
            if (x.length > 0 && (O = x[0]), O instanceof Error) throw O;
            var P = new Error("Unhandled error." + (O ? " (" + O.message + ")" : ""));
            throw P.context = O, P;
          }
          var A = W[f];
          if (A === void 0) return !1;
          if (typeof A == "function") l(A, this, x);
          else {
            var v = A.length, C = j(A, v);
            for (E = 0; E < v; ++E) l(C[E], this, x);
          }
          return !0;
        }, c.prototype.addListener = function(f, x) {
          return g(this, f, x, !1);
        }, c.prototype.on = c.prototype.addListener, c.prototype.prependListener = function(f, x) {
          return g(this, f, x, !0);
        }, c.prototype.once = function(f, x) {
          return p(x), this.on(f, y(this, f, x)), this;
        }, c.prototype.prependOnceListener = function(f, x) {
          return p(x), this.prependListener(f, y(this, f, x)), this;
        }, c.prototype.removeListener = function(f, x) {
          var E, L, W, O, P;
          if (p(x), (L = this._events) === void 0) return this;
          if ((E = L[f]) === void 0) return this;
          if (E === x || E.listener === x) --this._eventsCount == 0 ? this._events = /* @__PURE__ */ Object.create(null) : (delete L[f], L.removeListener && this.emit("removeListener", f, E.listener || x));
          else if (typeof E != "function") {
            for (W = -1, O = E.length - 1; O >= 0; O--) if (E[O] === x || E[O].listener === x) {
              P = E[O].listener, W = O;
              break;
            }
            if (W < 0) return this;
            W === 0 ? E.shift() : (function(A, v) {
              for (; v + 1 < A.length; v++) A[v] = A[v + 1];
              A.pop();
            })(E, W), E.length === 1 && (L[f] = E[0]), L.removeListener !== void 0 && this.emit("removeListener", f, P || x);
          }
          return this;
        }, c.prototype.off = c.prototype.removeListener, c.prototype.removeAllListeners = function(f) {
          var x, E, L;
          if ((E = this._events) === void 0) return this;
          if (E.removeListener === void 0) return arguments.length === 0 ? (this._events = /* @__PURE__ */ Object.create(null), this._eventsCount = 0) : E[f] !== void 0 && (--this._eventsCount == 0 ? this._events = /* @__PURE__ */ Object.create(null) : delete E[f]), this;
          if (arguments.length === 0) {
            var W, O = Object.keys(E);
            for (L = 0; L < O.length; ++L) (W = O[L]) !== "removeListener" && this.removeAllListeners(W);
            return this.removeAllListeners("removeListener"), this._events = /* @__PURE__ */ Object.create(null), this._eventsCount = 0, this;
          }
          if (typeof (x = E[f]) == "function") this.removeListener(f, x);
          else if (x !== void 0) for (L = x.length - 1; L >= 0; L--) this.removeListener(f, x[L]);
          return this;
        }, c.prototype.listeners = function(f) {
          return w(this, f, !0);
        }, c.prototype.rawListeners = function(f) {
          return w(this, f, !1);
        }, c.listenerCount = function(f, x) {
          return typeof f.listenerCount == "function" ? f.listenerCount(x) : k.call(f, x);
        }, c.prototype.listenerCount = k, c.prototype.eventNames = function() {
          return this._eventsCount > 0 ? i(this._events) : [];
        };
      }, function(r, n, o) {
        r.exports.Dispatcher = o(140);
      }, function(r, n, o) {
        r.exports = o(142);
      }, function(r, n, o) {
        n.__esModule = !0;
        var i = u(o(50)), s = u(o(65)), l = typeof s.default == "function" && typeof i.default == "symbol" ? function(c) {
          return typeof c;
        } : function(c) {
          return c && typeof s.default == "function" && c.constructor === s.default && c !== s.default.prototype ? "symbol" : typeof c;
        };
        function u(c) {
          return c && c.__esModule ? c : { default: c };
        }
        n.default = typeof s.default == "function" && l(i.default) === "symbol" ? function(c) {
          return c === void 0 ? "undefined" : l(c);
        } : function(c) {
          return c && typeof s.default == "function" && c.constructor === s.default && c !== s.default.prototype ? "symbol" : c === void 0 ? "undefined" : l(c);
        };
      }, function(r, n, o) {
        r.exports = { default: o(51), __esModule: !0 };
      }, function(r, n, o) {
        o(20), o(29), r.exports = o(30).f("iterator");
      }, function(r, n, o) {
        var i = o(21), s = o(22);
        r.exports = function(l) {
          return function(u, c) {
            var d, p, h = String(s(u)), g = i(c), m = h.length;
            return g < 0 || g >= m ? l ? "" : void 0 : (d = h.charCodeAt(g)) < 55296 || d > 56319 || g + 1 === m || (p = h.charCodeAt(g + 1)) < 56320 || p > 57343 ? l ? h.charAt(g) : d : l ? h.slice(g, g + 2) : p - 56320 + (d - 55296 << 10) + 65536;
          };
        };
      }, function(r, n, o) {
        var i = o(54);
        r.exports = function(s, l, u) {
          if (i(s), l === void 0) return s;
          switch (u) {
            case 1:
              return function(c) {
                return s.call(l, c);
              };
            case 2:
              return function(c, d) {
                return s.call(l, c, d);
              };
            case 3:
              return function(c, d, p) {
                return s.call(l, c, d, p);
              };
          }
          return function() {
            return s.apply(l, arguments);
          };
        };
      }, function(r, n) {
        r.exports = function(o) {
          if (typeof o != "function") throw TypeError(o + " is not a function!");
          return o;
        };
      }, function(r, n, o) {
        var i = o(38), s = o(16), l = o(28), u = {};
        o(6)(u, o(2)("iterator"), (function() {
          return this;
        })), r.exports = function(c, d, p) {
          c.prototype = i(u, { next: s(1, p) }), l(c, d + " Iterator");
        };
      }, function(r, n, o) {
        var i = o(7), s = o(10), l = o(13);
        r.exports = o(4) ? Object.defineProperties : function(u, c) {
          s(u);
          for (var d, p = l(c), h = p.length, g = 0; h > g; ) i.f(u, d = p[g++], c[d]);
          return u;
        };
      }, function(r, n, o) {
        var i = o(9), s = o(58), l = o(59);
        r.exports = function(u) {
          return function(c, d, p) {
            var h, g = i(c), m = s(g.length), y = l(p, m);
            if (u && d != d) {
              for (; m > y; ) if ((h = g[y++]) != h) return !0;
            } else for (; m > y; y++) if ((u || y in g) && g[y] === d) return u || y || 0;
            return !u && -1;
          };
        };
      }, function(r, n, o) {
        var i = o(21), s = Math.min;
        r.exports = function(l) {
          return l > 0 ? s(i(l), 9007199254740991) : 0;
        };
      }, function(r, n, o) {
        var i = o(21), s = Math.max, l = Math.min;
        r.exports = function(u, c) {
          return (u = i(u)) < 0 ? s(u + c, 0) : l(u, c);
        };
      }, function(r, n, o) {
        var i = o(3).document;
        r.exports = i && i.documentElement;
      }, function(r, n, o) {
        var i = o(5), s = o(18), l = o(25)("IE_PROTO"), u = Object.prototype;
        r.exports = Object.getPrototypeOf || function(c) {
          return c = s(c), i(c, l) ? c[l] : typeof c.constructor == "function" && c instanceof c.constructor ? c.constructor.prototype : c instanceof Object ? u : null;
        };
      }, function(r, n, o) {
        var i = o(63), s = o(64), l = o(12), u = o(9);
        r.exports = o(34)(Array, "Array", (function(c, d) {
          this._t = u(c), this._i = 0, this._k = d;
        }), (function() {
          var c = this._t, d = this._k, p = this._i++;
          return !c || p >= c.length ? (this._t = void 0, s(1)) : s(0, d == "keys" ? p : d == "values" ? c[p] : [p, c[p]]);
        }), "values"), l.Arguments = l.Array, i("keys"), i("values"), i("entries");
      }, function(r, n) {
        r.exports = function() {
        };
      }, function(r, n) {
        r.exports = function(o, i) {
          return { value: i, done: !!o };
        };
      }, function(r, n, o) {
        r.exports = { default: o(66), __esModule: !0 };
      }, function(r, n, o) {
        o(67), o(73), o(74), o(75), r.exports = o(1).Symbol;
      }, function(r, n, o) {
        var i = o(3), s = o(5), l = o(4), u = o(15), c = o(37), d = o(68).KEY, p = o(8), h = o(26), g = o(28), m = o(17), y = o(2), w = o(30), k = o(31), j = o(69), f = o(70), x = o(10), E = o(11), L = o(18), W = o(9), O = o(23), P = o(16), A = o(38), v = o(71), C = o(72), M = o(32), D = o(7), $ = o(13), H = C.f, F = D.f, q = v.f, z = i.Symbol, Z = i.JSON, Q = Z && Z.stringify, ue = y("_hidden"), de = y("toPrimitive"), Se = {}.propertyIsEnumerable, fe = h("symbol-registry"), ye = h("symbols"), be = h("op-symbols"), we = Object.prototype, Ee = typeof z == "function" && !!M.f, ge = i.QObject, ke = !ge || !ge.prototype || !ge.prototype.findChild, je = l && p((function() {
          return A(F({}, "a", { get: function() {
            return F(this, "a", { value: 7 }).a;
          } })).a != 7;
        })) ? function(U, te, oe) {
          var xe = H(we, te);
          xe && delete we[te], F(U, te, oe), xe && U !== we && F(we, te, xe);
        } : F, Te = function(U) {
          var te = ye[U] = A(z.prototype);
          return te._k = U, te;
        }, He = Ee && typeof z.iterator == "symbol" ? function(U) {
          return typeof U == "symbol";
        } : function(U) {
          return U instanceof z;
        }, _e = function(U, te, oe) {
          return U === we && _e(be, te, oe), x(U), te = O(te, !0), x(oe), s(ye, te) ? (oe.enumerable ? (s(U, ue) && U[ue][te] && (U[ue][te] = !1), oe = A(oe, { enumerable: P(0, !1) })) : (s(U, ue) || F(U, ue, P(1, {})), U[ue][te] = !0), je(U, te, oe)) : F(U, te, oe);
        }, Fe = function(U, te) {
          x(U);
          for (var oe, xe = j(te = W(te)), Me = 0, Ce = xe.length; Ce > Me; ) _e(U, oe = xe[Me++], te[oe]);
          return U;
        }, We = function(U) {
          var te = Se.call(this, U = O(U, !0));
          return !(this === we && s(ye, U) && !s(be, U)) && (!(te || !s(this, U) || !s(ye, U) || s(this, ue) && this[ue][U]) || te);
        }, Ue = function(U, te) {
          if (U = W(U), te = O(te, !0), U !== we || !s(ye, te) || s(be, te)) {
            var oe = H(U, te);
            return !oe || !s(ye, te) || s(U, ue) && U[ue][te] || (oe.enumerable = !0), oe;
          }
        }, tt = function(U) {
          for (var te, oe = q(W(U)), xe = [], Me = 0; oe.length > Me; ) s(ye, te = oe[Me++]) || te == ue || te == d || xe.push(te);
          return xe;
        }, Qe = function(U) {
          for (var te, oe = U === we, xe = q(oe ? be : W(U)), Me = [], Ce = 0; xe.length > Ce; ) !s(ye, te = xe[Ce++]) || oe && !s(we, te) || Me.push(ye[te]);
          return Me;
        };
        Ee || (c((z = function() {
          if (this instanceof z) throw TypeError("Symbol is not a constructor!");
          var U = m(arguments.length > 0 ? arguments[0] : void 0), te = function(oe) {
            this === we && te.call(be, oe), s(this, ue) && s(this[ue], U) && (this[ue][U] = !1), je(this, U, P(1, oe));
          };
          return l && ke && je(we, U, { configurable: !0, set: te }), Te(U);
        }).prototype, "toString", (function() {
          return this._k;
        })), C.f = Ue, D.f = _e, o(41).f = v.f = tt, o(19).f = We, M.f = Qe, l && !o(14) && c(we, "propertyIsEnumerable", We, !0), w.f = function(U) {
          return Te(y(U));
        }), u(u.G + u.W + u.F * !Ee, { Symbol: z });
        for (var Ve = "hasInstance,isConcatSpreadable,iterator,match,replace,search,species,split,toPrimitive,toStringTag,unscopables".split(","), ce = 0; Ve.length > ce; ) y(Ve[ce++]);
        for (var pe = $(y.store), X = 0; pe.length > X; ) k(pe[X++]);
        u(u.S + u.F * !Ee, "Symbol", { for: function(U) {
          return s(fe, U += "") ? fe[U] : fe[U] = z(U);
        }, keyFor: function(U) {
          if (!He(U)) throw TypeError(U + " is not a symbol!");
          for (var te in fe) if (fe[te] === U) return te;
        }, useSetter: function() {
          ke = !0;
        }, useSimple: function() {
          ke = !1;
        } }), u(u.S + u.F * !Ee, "Object", { create: function(U, te) {
          return te === void 0 ? A(U) : Fe(A(U), te);
        }, defineProperty: _e, defineProperties: Fe, getOwnPropertyDescriptor: Ue, getOwnPropertyNames: tt, getOwnPropertySymbols: Qe });
        var V = p((function() {
          M.f(1);
        }));
        u(u.S + u.F * V, "Object", { getOwnPropertySymbols: function(U) {
          return M.f(L(U));
        } }), Z && u(u.S + u.F * (!Ee || p((function() {
          var U = z();
          return Q([U]) != "[null]" || Q({ a: U }) != "{}" || Q(Object(U)) != "{}";
        }))), "JSON", { stringify: function(U) {
          for (var te, oe, xe = [U], Me = 1; arguments.length > Me; ) xe.push(arguments[Me++]);
          if (oe = te = xe[1], (E(te) || U !== void 0) && !He(U)) return f(te) || (te = function(Ce, me) {
            if (typeof oe == "function" && (me = oe.call(this, Ce, me)), !He(me)) return me;
          }), xe[1] = te, Q.apply(Z, xe);
        } }), z.prototype[de] || o(6)(z.prototype, de, z.prototype.valueOf), g(z, "Symbol"), g(Math, "Math", !0), g(i.JSON, "JSON", !0);
      }, function(r, n, o) {
        var i = o(17)("meta"), s = o(11), l = o(5), u = o(7).f, c = 0, d = Object.isExtensible || function() {
          return !0;
        }, p = !o(8)((function() {
          return d(Object.preventExtensions({}));
        })), h = function(m) {
          u(m, i, { value: { i: "O" + ++c, w: {} } });
        }, g = r.exports = { KEY: i, NEED: !1, fastKey: function(m, y) {
          if (!s(m)) return typeof m == "symbol" ? m : (typeof m == "string" ? "S" : "P") + m;
          if (!l(m, i)) {
            if (!d(m)) return "F";
            if (!y) return "E";
            h(m);
          }
          return m[i].i;
        }, getWeak: function(m, y) {
          if (!l(m, i)) {
            if (!d(m)) return !0;
            if (!y) return !1;
            h(m);
          }
          return m[i].w;
        }, onFreeze: function(m) {
          return p && g.NEED && d(m) && !l(m, i) && h(m), m;
        } };
      }, function(r, n, o) {
        var i = o(13), s = o(32), l = o(19);
        r.exports = function(u) {
          var c = i(u), d = s.f;
          if (d) for (var p, h = d(u), g = l.f, m = 0; h.length > m; ) g.call(u, p = h[m++]) && c.push(p);
          return c;
        };
      }, function(r, n, o) {
        var i = o(24);
        r.exports = Array.isArray || function(s) {
          return i(s) == "Array";
        };
      }, function(r, n, o) {
        var i = o(9), s = o(41).f, l = {}.toString, u = typeof window == "object" && window && Object.getOwnPropertyNames ? Object.getOwnPropertyNames(window) : [];
        r.exports.f = function(c) {
          return u && l.call(c) == "[object Window]" ? (function(d) {
            try {
              return s(d);
            } catch {
              return u.slice();
            }
          })(c) : s(i(c));
        };
      }, function(r, n, o) {
        var i = o(19), s = o(16), l = o(9), u = o(23), c = o(5), d = o(35), p = Object.getOwnPropertyDescriptor;
        n.f = o(4) ? p : function(h, g) {
          if (h = l(h), g = u(g, !0), d) try {
            return p(h, g);
          } catch {
          }
          if (c(h, g)) return s(!i.f.call(h, g), h[g]);
        };
      }, function(r, n) {
      }, function(r, n, o) {
        o(31)("asyncIterator");
      }, function(r, n, o) {
        o(31)("observable");
      }, function(r, n, o) {
        n.__esModule = !0;
        var i, s = o(77), l = (i = s) && i.__esModule ? i : { default: i };
        n.default = l.default || function(u) {
          for (var c = 1; c < arguments.length; c++) {
            var d = arguments[c];
            for (var p in d) Object.prototype.hasOwnProperty.call(d, p) && (u[p] = d[p]);
          }
          return u;
        };
      }, function(r, n, o) {
        r.exports = { default: o(78), __esModule: !0 };
      }, function(r, n, o) {
        o(79), r.exports = o(1).Object.assign;
      }, function(r, n, o) {
        var i = o(15);
        i(i.S + i.F, "Object", { assign: o(80) });
      }, function(r, n, o) {
        var i = o(4), s = o(13), l = o(32), u = o(19), c = o(18), d = o(40), p = Object.assign;
        r.exports = !p || o(8)((function() {
          var h = {}, g = {}, m = Symbol(), y = "abcdefghijklmnopqrst";
          return h[m] = 7, y.split("").forEach((function(w) {
            g[w] = w;
          })), p({}, h)[m] != 7 || Object.keys(p({}, g)).join("") != y;
        })) ? function(h, g) {
          for (var m = c(h), y = arguments.length, w = 1, k = l.f, j = u.f; y > w; ) for (var f, x = d(arguments[w++]), E = k ? s(x).concat(k(x)) : s(x), L = E.length, W = 0; L > W; ) f = E[W++], i && !j.call(x, f) || (m[f] = x[f]);
          return m;
        } : p;
      }, function(r, n, o) {
        n.__esModule = !0;
        var i = l(o(82)), s = l(o(85));
        function l(u) {
          return u && u.__esModule ? u : { default: u };
        }
        n.default = function(u, c) {
          if (Array.isArray(u)) return u;
          if ((0, i.default)(Object(u))) return (function(d, p) {
            var h = [], g = !0, m = !1, y = void 0;
            try {
              for (var w, k = (0, s.default)(d); !(g = (w = k.next()).done) && (h.push(w.value), !p || h.length !== p); g = !0) ;
            } catch (j) {
              m = !0, y = j;
            } finally {
              try {
                !g && k.return && k.return();
              } finally {
                if (m) throw y;
              }
            }
            return h;
          })(u, c);
          throw new TypeError("Invalid attempt to destructure non-iterable instance");
        };
      }, function(r, n, o) {
        r.exports = { default: o(83), __esModule: !0 };
      }, function(r, n, o) {
        o(29), o(20), r.exports = o(84);
      }, function(r, n, o) {
        var i = o(42), s = o(2)("iterator"), l = o(12);
        r.exports = o(1).isIterable = function(u) {
          var c = Object(u);
          return c[s] !== void 0 || "@@iterator" in c || l.hasOwnProperty(i(c));
        };
      }, function(r, n, o) {
        r.exports = { default: o(86), __esModule: !0 };
      }, function(r, n, o) {
        o(29), o(20), r.exports = o(87);
      }, function(r, n, o) {
        var i = o(10), s = o(88);
        r.exports = o(1).getIterator = function(l) {
          var u = s(l);
          if (typeof u != "function") throw TypeError(l + " is not iterable!");
          return i(u.call(l));
        };
      }, function(r, n, o) {
        var i = o(42), s = o(2)("iterator"), l = o(12);
        r.exports = o(1).getIteratorMethod = function(u) {
          if (u != null) return u[s] || u["@@iterator"] || l[i(u)];
        };
      }, function(r, n, o) {
        r.exports = { default: o(90), __esModule: !0 };
      }, function(r, n, o) {
        o(91), r.exports = o(1).Object.keys;
      }, function(r, n, o) {
        var i = o(18), s = o(13);
        o(92)("keys", (function() {
          return function(l) {
            return s(i(l));
          };
        }));
      }, function(r, n, o) {
        var i = o(15), s = o(1), l = o(8);
        r.exports = function(u, c) {
          var d = (s.Object || {})[u] || Object[u], p = {};
          p[u] = c(d), i(i.S + i.F * l((function() {
            d(1);
          })), "Object", p);
        };
      }, function(r, n, o) {
        (function(i) {
          var s = [["ary", 128], ["bind", 1], ["bindKey", 2], ["curry", 8], ["curryRight", 16], ["flip", 512], ["partial", 32], ["partialRight", 64], ["rearg", 256]], l = /^\s+|\s+$/g, u = /\{(?:\n\/\* \[wrapped with .+\] \*\/)?\n?/, c = /\{\n\/\* \[wrapped with (.+)\] \*/, d = /,? & /, p = /^[-+]0x[0-9a-f]+$/i, h = /^0b[01]+$/i, g = /^\[object .+?Constructor\]$/, m = /^0o[0-7]+$/i, y = /^(?:0|[1-9]\d*)$/, w = parseInt, k = typeof i == "object" && i && i.Object === Object && i, j = typeof self == "object" && self && self.Object === Object && self, f = k || j || Function("return this")();
          function x(X, V, U) {
            switch (U.length) {
              case 0:
                return X.call(V);
              case 1:
                return X.call(V, U[0]);
              case 2:
                return X.call(V, U[0], U[1]);
              case 3:
                return X.call(V, U[0], U[1], U[2]);
            }
            return X.apply(V, U);
          }
          function E(X, V) {
            return !!(X && X.length) && (function(U, te, oe) {
              if (te != te) return (function(Ce, me, De, ze) {
                for (var Pe = Ce.length, ie = De + -1; ++ie < Pe; ) if (me(Ce[ie], ie, Ce)) return ie;
                return -1;
              })(U, L, oe);
              for (var xe = oe - 1, Me = U.length; ++xe < Me; ) if (U[xe] === te) return xe;
              return -1;
            })(X, V, 0) > -1;
          }
          function L(X) {
            return X != X;
          }
          function W(X, V) {
            for (var U = X.length, te = 0; U--; ) X[U] === V && te++;
            return te;
          }
          function O(X, V) {
            for (var U = -1, te = X.length, oe = 0, xe = []; ++U < te; ) {
              var Me = X[U];
              Me !== V && Me !== "__lodash_placeholder__" || (X[U] = "__lodash_placeholder__", xe[oe++] = U);
            }
            return xe;
          }
          var P, A, v, C = Function.prototype, M = Object.prototype, D = f["__core-js_shared__"], $ = (P = /[^.]+$/.exec(D && D.keys && D.keys.IE_PROTO || "")) ? "Symbol(src)_1." + P : "", H = C.toString, F = M.hasOwnProperty, q = M.toString, z = RegExp("^" + H.call(F).replace(/[\\^$.*+?()[\]{}|]/g, "\\$&").replace(/hasOwnProperty|(function).*?(?=\\\()| for .+?(?=\\\])/g, "$1.*?") + "$"), Z = Object.create, Q = Math.max, ue = Math.min, de = (A = Te(Object, "defineProperty"), (v = Te.name) && v.length > 2 ? A : void 0);
          function Se(X) {
            return Ve(X) ? Z(X) : {};
          }
          function fe(X) {
            return !(!Ve(X) || (function(V) {
              return !!$ && $ in V;
            })(X)) && ((function(V) {
              var U = Ve(V) ? q.call(V) : "";
              return U == "[object Function]" || U == "[object GeneratorFunction]";
            })(X) || (function(V) {
              var U = !1;
              if (V != null && typeof V.toString != "function") try {
                U = !!(V + "");
              } catch {
              }
              return U;
            })(X) ? z : g).test((function(V) {
              if (V != null) {
                try {
                  return H.call(V);
                } catch {
                }
                try {
                  return V + "";
                } catch {
                }
              }
              return "";
            })(X));
          }
          function ye(X, V, U, te) {
            for (var oe = -1, xe = X.length, Me = U.length, Ce = -1, me = V.length, De = Q(xe - Me, 0), ze = Array(me + De), Pe = !te; ++Ce < me; ) ze[Ce] = V[Ce];
            for (; ++oe < Me; ) (Pe || oe < xe) && (ze[U[oe]] = X[oe]);
            for (; De--; ) ze[Ce++] = X[oe++];
            return ze;
          }
          function be(X, V, U, te) {
            for (var oe = -1, xe = X.length, Me = -1, Ce = U.length, me = -1, De = V.length, ze = Q(xe - Ce, 0), Pe = Array(ze + De), ie = !te; ++oe < ze; ) Pe[oe] = X[oe];
            for (var $e = oe; ++me < De; ) Pe[$e + me] = V[me];
            for (; ++Me < Ce; ) (ie || oe < xe) && (Pe[$e + U[Me]] = X[oe++]);
            return Pe;
          }
          function we(X) {
            return function() {
              var V = arguments;
              switch (V.length) {
                case 0:
                  return new X();
                case 1:
                  return new X(V[0]);
                case 2:
                  return new X(V[0], V[1]);
                case 3:
                  return new X(V[0], V[1], V[2]);
                case 4:
                  return new X(V[0], V[1], V[2], V[3]);
                case 5:
                  return new X(V[0], V[1], V[2], V[3], V[4]);
                case 6:
                  return new X(V[0], V[1], V[2], V[3], V[4], V[5]);
                case 7:
                  return new X(V[0], V[1], V[2], V[3], V[4], V[5], V[6]);
              }
              var U = Se(X.prototype), te = X.apply(U, V);
              return Ve(te) ? te : U;
            };
          }
          function Ee(X, V, U, te, oe, xe, Me, Ce, me, De) {
            var ze = 128 & V, Pe = 1 & V, ie = 2 & V, $e = 24 & V, qe = 512 & V, Je = ie ? void 0 : we(X);
            return function at() {
              for (var Ne = arguments.length, Le = Array(Ne), mt = Ne; mt--; ) Le[mt] = arguments[mt];
              if ($e) var ot = je(at), rt = W(Le, ot);
              if (te && (Le = ye(Le, te, oe, $e)), xe && (Le = be(Le, xe, Me, $e)), Ne -= rt, $e && Ne < De) {
                var Ze = O(Le, ot);
                return ge(X, V, Ee, at.placeholder, U, Le, Ze, Ce, me, De - Ne);
              }
              var bt = Pe ? U : this, Nt = ie ? bt[X] : X;
              return Ne = Le.length, Ce ? Le = We(Le, Ce) : qe && Ne > 1 && Le.reverse(), ze && me < Ne && (Le.length = me), this && this !== f && this instanceof at && (Nt = Je || we(Nt)), Nt.apply(bt, Le);
            };
          }
          function ge(X, V, U, te, oe, xe, Me, Ce, me, De) {
            var ze = 8 & V;
            V |= ze ? 32 : 64, 4 & (V &= ~(ze ? 64 : 32)) || (V &= -4);
            var Pe = U(X, V, oe, ze ? xe : void 0, ze ? Me : void 0, ze ? void 0 : xe, ze ? void 0 : Me, Ce, me, De);
            return Pe.placeholder = te, Ue(Pe, X, V);
          }
          function ke(X, V, U, te, oe, xe, Me, Ce) {
            var me = 2 & V;
            if (!me && typeof X != "function") throw new TypeError("Expected a function");
            var De = te ? te.length : 0;
            if (De || (V &= -97, te = oe = void 0), Me = Me === void 0 ? Me : Q(pe(Me), 0), Ce = Ce === void 0 ? Ce : pe(Ce), De -= oe ? oe.length : 0, 64 & V) {
              var ze = te, Pe = oe;
              te = oe = void 0;
            }
            var ie = [X, V, U, te, oe, ze, Pe, xe, Me, Ce];
            if (X = ie[0], V = ie[1], U = ie[2], te = ie[3], oe = ie[4], !(Ce = ie[9] = ie[9] == null ? me ? 0 : X.length : Q(ie[9] - De, 0)) && 24 & V && (V &= -25), V && V != 1) $e = V == 8 || V == 16 ? (function(qe, Je, at) {
              var Ne = we(qe);
              return function Le() {
                for (var mt = arguments.length, ot = Array(mt), rt = mt, Ze = je(Le); rt--; ) ot[rt] = arguments[rt];
                var bt = mt < 3 && ot[0] !== Ze && ot[mt - 1] !== Ze ? [] : O(ot, Ze);
                if ((mt -= bt.length) < at) return ge(qe, Je, Ee, Le.placeholder, void 0, ot, bt, void 0, void 0, at - mt);
                var Nt = this && this !== f && this instanceof Le ? Ne : qe;
                return x(Nt, this, ot);
              };
            })(X, V, Ce) : V != 32 && V != 33 || oe.length ? Ee.apply(void 0, ie) : (function(qe, Je, at, Ne) {
              var Le = 1 & Je, mt = we(qe);
              return function ot() {
                for (var rt = -1, Ze = arguments.length, bt = -1, Nt = Ne.length, Lt = Array(Nt + Ze), _r = this && this !== f && this instanceof ot ? mt : qe; ++bt < Nt; ) Lt[bt] = Ne[bt];
                for (; Ze--; ) Lt[bt++] = arguments[++rt];
                return x(_r, Le ? at : this, Lt);
              };
            })(X, V, U, te);
            else var $e = (function(qe, Je, at) {
              var Ne = 1 & Je, Le = we(qe);
              return function mt() {
                var ot = this && this !== f && this instanceof mt ? Le : qe;
                return ot.apply(Ne ? at : this, arguments);
              };
            })(X, V, U);
            return Ue($e, X, V);
          }
          function je(X) {
            return X.placeholder;
          }
          function Te(X, V) {
            var U = (function(te, oe) {
              return te == null ? void 0 : te[oe];
            })(X, V);
            return fe(U) ? U : void 0;
          }
          function He(X) {
            var V = X.match(c);
            return V ? V[1].split(d) : [];
          }
          function _e(X, V) {
            var U = V.length, te = U - 1;
            return V[te] = (U > 1 ? "& " : "") + V[te], V = V.join(U > 2 ? ", " : " "), X.replace(u, `{
/* [wrapped with ` + V + `] */
`);
          }
          function Fe(X, V) {
            return !!(V = V ?? 9007199254740991) && (typeof X == "number" || y.test(X)) && X > -1 && X % 1 == 0 && X < V;
          }
          function We(X, V) {
            for (var U = X.length, te = ue(V.length, U), oe = (function(Me, Ce) {
              var me = -1, De = Me.length;
              for (Ce || (Ce = Array(De)); ++me < De; ) Ce[me] = Me[me];
              return Ce;
            })(X); te--; ) {
              var xe = V[te];
              X[te] = Fe(xe, U) ? oe[xe] : void 0;
            }
            return X;
          }
          var Ue = de ? function(X, V, U) {
            var te, oe = V + "";
            return de(X, "toString", { configurable: !0, enumerable: !1, value: (te = _e(oe, tt(He(oe), U)), function() {
              return te;
            }) });
          } : function(X) {
            return X;
          };
          function tt(X, V) {
            return (function(U, te) {
              for (var oe = -1, xe = U ? U.length : 0; ++oe < xe && te(U[oe], oe, U) !== !1; ) ;
            })(s, (function(U) {
              var te = "_." + U[0];
              V & U[1] && !E(X, te) && X.push(te);
            })), X.sort();
          }
          function Qe(X, V, U) {
            var te = ke(X, 8, void 0, void 0, void 0, void 0, void 0, V = U ? void 0 : V);
            return te.placeholder = Qe.placeholder, te;
          }
          function Ve(X) {
            var V = typeof X;
            return !!X && (V == "object" || V == "function");
          }
          function ce(X) {
            return X ? (X = (function(V) {
              if (typeof V == "number") return V;
              if ((function(oe) {
                return typeof oe == "symbol" || /* @__PURE__ */ (function(xe) {
                  return !!xe && typeof xe == "object";
                })(oe) && q.call(oe) == "[object Symbol]";
              })(V)) return NaN;
              if (Ve(V)) {
                var U = typeof V.valueOf == "function" ? V.valueOf() : V;
                V = Ve(U) ? U + "" : U;
              }
              if (typeof V != "string") return V === 0 ? V : +V;
              V = V.replace(l, "");
              var te = h.test(V);
              return te || m.test(V) ? w(V.slice(2), te ? 2 : 8) : p.test(V) ? NaN : +V;
            })(X)) === 1 / 0 || X === -1 / 0 ? 17976931348623157e292 * (X < 0 ? -1 : 1) : X == X ? X : 0 : X === 0 ? X : 0;
          }
          function pe(X) {
            var V = ce(X), U = V % 1;
            return V == V ? U ? V - U : V : 0;
          }
          Qe.placeholder = {}, r.exports = Qe;
        }).call(this, o(43));
      }, function(r, n, o) {
        function i(be) {
          return be && be.__esModule ? be.default : be;
        }
        n.__esModule = !0;
        var s = o(95);
        n.threezerotwofour = i(s);
        var l = o(96);
        n.apathy = i(l);
        var u = o(97);
        n.ashes = i(u);
        var c = o(98);
        n.atelierDune = i(c);
        var d = o(99);
        n.atelierForest = i(d);
        var p = o(100);
        n.atelierHeath = i(p);
        var h = o(101);
        n.atelierLakeside = i(h);
        var g = o(102);
        n.atelierSeaside = i(g);
        var m = o(103);
        n.bespin = i(m);
        var y = o(104);
        n.brewer = i(y);
        var w = o(105);
        n.bright = i(w);
        var k = o(106);
        n.chalk = i(k);
        var j = o(107);
        n.codeschool = i(j);
        var f = o(108);
        n.colors = i(f);
        var x = o(109);
        n.default = i(x);
        var E = o(110);
        n.eighties = i(E);
        var L = o(111);
        n.embers = i(L);
        var W = o(112);
        n.flat = i(W);
        var O = o(113);
        n.google = i(O);
        var P = o(114);
        n.grayscale = i(P);
        var A = o(115);
        n.greenscreen = i(A);
        var v = o(116);
        n.harmonic = i(v);
        var C = o(117);
        n.hopscotch = i(C);
        var M = o(118);
        n.isotope = i(M);
        var D = o(119);
        n.marrakesh = i(D);
        var $ = o(120);
        n.mocha = i($);
        var H = o(121);
        n.monokai = i(H);
        var F = o(122);
        n.ocean = i(F);
        var q = o(123);
        n.paraiso = i(q);
        var z = o(124);
        n.pop = i(z);
        var Z = o(125);
        n.railscasts = i(Z);
        var Q = o(126);
        n.shapeshifter = i(Q);
        var ue = o(127);
        n.solarized = i(ue);
        var de = o(128);
        n.summerfruit = i(de);
        var Se = o(129);
        n.tomorrow = i(Se);
        var fe = o(130);
        n.tube = i(fe);
        var ye = o(131);
        n.twilight = i(ye);
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "threezerotwofour", author: "jan t. sott (http://github.com/idleberg)", base00: "#090300", base01: "#3a3432", base02: "#4a4543", base03: "#5c5855", base04: "#807d7c", base05: "#a5a2a2", base06: "#d6d5d4", base07: "#f7f7f7", base08: "#db2d20", base09: "#e8bbd0", base0A: "#fded02", base0B: "#01a252", base0C: "#b5e4f4", base0D: "#01a0e4", base0E: "#a16a94", base0F: "#cdab53" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "apathy", author: "jannik siebert (https://github.com/janniks)", base00: "#031A16", base01: "#0B342D", base02: "#184E45", base03: "#2B685E", base04: "#5F9C92", base05: "#81B5AC", base06: "#A7CEC8", base07: "#D2E7E4", base08: "#3E9688", base09: "#3E7996", base0A: "#3E4C96", base0B: "#883E96", base0C: "#963E4C", base0D: "#96883E", base0E: "#4C963E", base0F: "#3E965B" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "ashes", author: "jannik siebert (https://github.com/janniks)", base00: "#1C2023", base01: "#393F45", base02: "#565E65", base03: "#747C84", base04: "#ADB3BA", base05: "#C7CCD1", base06: "#DFE2E5", base07: "#F3F4F5", base08: "#C7AE95", base09: "#C7C795", base0A: "#AEC795", base0B: "#95C7AE", base0C: "#95AEC7", base0D: "#AE95C7", base0E: "#C795AE", base0F: "#C79595" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "atelier dune", author: "bram de haan (http://atelierbram.github.io/syntax-highlighting/atelier-schemes/dune)", base00: "#20201d", base01: "#292824", base02: "#6e6b5e", base03: "#7d7a68", base04: "#999580", base05: "#a6a28c", base06: "#e8e4cf", base07: "#fefbec", base08: "#d73737", base09: "#b65611", base0A: "#cfb017", base0B: "#60ac39", base0C: "#1fad83", base0D: "#6684e1", base0E: "#b854d4", base0F: "#d43552" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "atelier forest", author: "bram de haan (http://atelierbram.github.io/syntax-highlighting/atelier-schemes/forest)", base00: "#1b1918", base01: "#2c2421", base02: "#68615e", base03: "#766e6b", base04: "#9c9491", base05: "#a8a19f", base06: "#e6e2e0", base07: "#f1efee", base08: "#f22c40", base09: "#df5320", base0A: "#d5911a", base0B: "#5ab738", base0C: "#00ad9c", base0D: "#407ee7", base0E: "#6666ea", base0F: "#c33ff3" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "atelier heath", author: "bram de haan (http://atelierbram.github.io/syntax-highlighting/atelier-schemes/heath)", base00: "#1b181b", base01: "#292329", base02: "#695d69", base03: "#776977", base04: "#9e8f9e", base05: "#ab9bab", base06: "#d8cad8", base07: "#f7f3f7", base08: "#ca402b", base09: "#a65926", base0A: "#bb8a35", base0B: "#379a37", base0C: "#159393", base0D: "#516aec", base0E: "#7b59c0", base0F: "#cc33cc" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "atelier lakeside", author: "bram de haan (http://atelierbram.github.io/syntax-highlighting/atelier-schemes/lakeside/)", base00: "#161b1d", base01: "#1f292e", base02: "#516d7b", base03: "#5a7b8c", base04: "#7195a8", base05: "#7ea2b4", base06: "#c1e4f6", base07: "#ebf8ff", base08: "#d22d72", base09: "#935c25", base0A: "#8a8a0f", base0B: "#568c3b", base0C: "#2d8f6f", base0D: "#257fad", base0E: "#5d5db1", base0F: "#b72dd2" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "atelier seaside", author: "bram de haan (http://atelierbram.github.io/syntax-highlighting/atelier-schemes/seaside/)", base00: "#131513", base01: "#242924", base02: "#5e6e5e", base03: "#687d68", base04: "#809980", base05: "#8ca68c", base06: "#cfe8cf", base07: "#f0fff0", base08: "#e6193c", base09: "#87711d", base0A: "#c3c322", base0B: "#29a329", base0C: "#1999b3", base0D: "#3d62f5", base0E: "#ad2bee", base0F: "#e619c3" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "bespin", author: "jan t. sott", base00: "#28211c", base01: "#36312e", base02: "#5e5d5c", base03: "#666666", base04: "#797977", base05: "#8a8986", base06: "#9d9b97", base07: "#baae9e", base08: "#cf6a4c", base09: "#cf7d34", base0A: "#f9ee98", base0B: "#54be0d", base0C: "#afc4db", base0D: "#5ea6ea", base0E: "#9b859d", base0F: "#937121" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "brewer", author: "timothée poisot (http://github.com/tpoisot)", base00: "#0c0d0e", base01: "#2e2f30", base02: "#515253", base03: "#737475", base04: "#959697", base05: "#b7b8b9", base06: "#dadbdc", base07: "#fcfdfe", base08: "#e31a1c", base09: "#e6550d", base0A: "#dca060", base0B: "#31a354", base0C: "#80b1d3", base0D: "#3182bd", base0E: "#756bb1", base0F: "#b15928" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "bright", author: "chris kempson (http://chriskempson.com)", base00: "#000000", base01: "#303030", base02: "#505050", base03: "#b0b0b0", base04: "#d0d0d0", base05: "#e0e0e0", base06: "#f5f5f5", base07: "#ffffff", base08: "#fb0120", base09: "#fc6d24", base0A: "#fda331", base0B: "#a1c659", base0C: "#76c7b7", base0D: "#6fb3d2", base0E: "#d381c3", base0F: "#be643c" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "chalk", author: "chris kempson (http://chriskempson.com)", base00: "#151515", base01: "#202020", base02: "#303030", base03: "#505050", base04: "#b0b0b0", base05: "#d0d0d0", base06: "#e0e0e0", base07: "#f5f5f5", base08: "#fb9fb1", base09: "#eda987", base0A: "#ddb26f", base0B: "#acc267", base0C: "#12cfc0", base0D: "#6fc2ef", base0E: "#e1a3ee", base0F: "#deaf8f" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "codeschool", author: "brettof86", base00: "#232c31", base01: "#1c3657", base02: "#2a343a", base03: "#3f4944", base04: "#84898c", base05: "#9ea7a6", base06: "#a7cfa3", base07: "#b5d8f6", base08: "#2a5491", base09: "#43820d", base0A: "#a03b1e", base0B: "#237986", base0C: "#b02f30", base0D: "#484d79", base0E: "#c59820", base0F: "#c98344" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "colors", author: "mrmrs (http://clrs.cc)", base00: "#111111", base01: "#333333", base02: "#555555", base03: "#777777", base04: "#999999", base05: "#bbbbbb", base06: "#dddddd", base07: "#ffffff", base08: "#ff4136", base09: "#ff851b", base0A: "#ffdc00", base0B: "#2ecc40", base0C: "#7fdbff", base0D: "#0074d9", base0E: "#b10dc9", base0F: "#85144b" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "default", author: "chris kempson (http://chriskempson.com)", base00: "#181818", base01: "#282828", base02: "#383838", base03: "#585858", base04: "#b8b8b8", base05: "#d8d8d8", base06: "#e8e8e8", base07: "#f8f8f8", base08: "#ab4642", base09: "#dc9656", base0A: "#f7ca88", base0B: "#a1b56c", base0C: "#86c1b9", base0D: "#7cafc2", base0E: "#ba8baf", base0F: "#a16946" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "eighties", author: "chris kempson (http://chriskempson.com)", base00: "#2d2d2d", base01: "#393939", base02: "#515151", base03: "#747369", base04: "#a09f93", base05: "#d3d0c8", base06: "#e8e6df", base07: "#f2f0ec", base08: "#f2777a", base09: "#f99157", base0A: "#ffcc66", base0B: "#99cc99", base0C: "#66cccc", base0D: "#6699cc", base0E: "#cc99cc", base0F: "#d27b53" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "embers", author: "jannik siebert (https://github.com/janniks)", base00: "#16130F", base01: "#2C2620", base02: "#433B32", base03: "#5A5047", base04: "#8A8075", base05: "#A39A90", base06: "#BEB6AE", base07: "#DBD6D1", base08: "#826D57", base09: "#828257", base0A: "#6D8257", base0B: "#57826D", base0C: "#576D82", base0D: "#6D5782", base0E: "#82576D", base0F: "#825757" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "flat", author: "chris kempson (http://chriskempson.com)", base00: "#2C3E50", base01: "#34495E", base02: "#7F8C8D", base03: "#95A5A6", base04: "#BDC3C7", base05: "#e0e0e0", base06: "#f5f5f5", base07: "#ECF0F1", base08: "#E74C3C", base09: "#E67E22", base0A: "#F1C40F", base0B: "#2ECC71", base0C: "#1ABC9C", base0D: "#3498DB", base0E: "#9B59B6", base0F: "#be643c" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "google", author: "seth wright (http://sethawright.com)", base00: "#1d1f21", base01: "#282a2e", base02: "#373b41", base03: "#969896", base04: "#b4b7b4", base05: "#c5c8c6", base06: "#e0e0e0", base07: "#ffffff", base08: "#CC342B", base09: "#F96A38", base0A: "#FBA922", base0B: "#198844", base0C: "#3971ED", base0D: "#3971ED", base0E: "#A36AC7", base0F: "#3971ED" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "grayscale", author: "alexandre gavioli (https://github.com/alexx2/)", base00: "#101010", base01: "#252525", base02: "#464646", base03: "#525252", base04: "#ababab", base05: "#b9b9b9", base06: "#e3e3e3", base07: "#f7f7f7", base08: "#7c7c7c", base09: "#999999", base0A: "#a0a0a0", base0B: "#8e8e8e", base0C: "#868686", base0D: "#686868", base0E: "#747474", base0F: "#5e5e5e" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "green screen", author: "chris kempson (http://chriskempson.com)", base00: "#001100", base01: "#003300", base02: "#005500", base03: "#007700", base04: "#009900", base05: "#00bb00", base06: "#00dd00", base07: "#00ff00", base08: "#007700", base09: "#009900", base0A: "#007700", base0B: "#00bb00", base0C: "#005500", base0D: "#009900", base0E: "#00bb00", base0F: "#005500" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "harmonic16", author: "jannik siebert (https://github.com/janniks)", base00: "#0b1c2c", base01: "#223b54", base02: "#405c79", base03: "#627e99", base04: "#aabcce", base05: "#cbd6e2", base06: "#e5ebf1", base07: "#f7f9fb", base08: "#bf8b56", base09: "#bfbf56", base0A: "#8bbf56", base0B: "#56bf8b", base0C: "#568bbf", base0D: "#8b56bf", base0E: "#bf568b", base0F: "#bf5656" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "hopscotch", author: "jan t. sott", base00: "#322931", base01: "#433b42", base02: "#5c545b", base03: "#797379", base04: "#989498", base05: "#b9b5b8", base06: "#d5d3d5", base07: "#ffffff", base08: "#dd464c", base09: "#fd8b19", base0A: "#fdcc59", base0B: "#8fc13e", base0C: "#149b93", base0D: "#1290bf", base0E: "#c85e7c", base0F: "#b33508" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "isotope", author: "jan t. sott", base00: "#000000", base01: "#404040", base02: "#606060", base03: "#808080", base04: "#c0c0c0", base05: "#d0d0d0", base06: "#e0e0e0", base07: "#ffffff", base08: "#ff0000", base09: "#ff9900", base0A: "#ff0099", base0B: "#33ff00", base0C: "#00ffff", base0D: "#0066ff", base0E: "#cc00ff", base0F: "#3300ff" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "marrakesh", author: "alexandre gavioli (http://github.com/alexx2/)", base00: "#201602", base01: "#302e00", base02: "#5f5b17", base03: "#6c6823", base04: "#86813b", base05: "#948e48", base06: "#ccc37a", base07: "#faf0a5", base08: "#c35359", base09: "#b36144", base0A: "#a88339", base0B: "#18974e", base0C: "#75a738", base0D: "#477ca1", base0E: "#8868b3", base0F: "#b3588e" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "mocha", author: "chris kempson (http://chriskempson.com)", base00: "#3B3228", base01: "#534636", base02: "#645240", base03: "#7e705a", base04: "#b8afad", base05: "#d0c8c6", base06: "#e9e1dd", base07: "#f5eeeb", base08: "#cb6077", base09: "#d28b71", base0A: "#f4bc87", base0B: "#beb55b", base0C: "#7bbda4", base0D: "#8ab3b5", base0E: "#a89bb9", base0F: "#bb9584" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "monokai", author: "wimer hazenberg (http://www.monokai.nl)", base00: "#272822", base01: "#383830", base02: "#49483e", base03: "#75715e", base04: "#a59f85", base05: "#f8f8f2", base06: "#f5f4f1", base07: "#f9f8f5", base08: "#f92672", base09: "#fd971f", base0A: "#f4bf75", base0B: "#a6e22e", base0C: "#a1efe4", base0D: "#66d9ef", base0E: "#ae81ff", base0F: "#cc6633" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "ocean", author: "chris kempson (http://chriskempson.com)", base00: "#2b303b", base01: "#343d46", base02: "#4f5b66", base03: "#65737e", base04: "#a7adba", base05: "#c0c5ce", base06: "#dfe1e8", base07: "#eff1f5", base08: "#bf616a", base09: "#d08770", base0A: "#ebcb8b", base0B: "#a3be8c", base0C: "#96b5b4", base0D: "#8fa1b3", base0E: "#b48ead", base0F: "#ab7967" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "paraiso", author: "jan t. sott", base00: "#2f1e2e", base01: "#41323f", base02: "#4f424c", base03: "#776e71", base04: "#8d8687", base05: "#a39e9b", base06: "#b9b6b0", base07: "#e7e9db", base08: "#ef6155", base09: "#f99b15", base0A: "#fec418", base0B: "#48b685", base0C: "#5bc4bf", base0D: "#06b6ef", base0E: "#815ba4", base0F: "#e96ba8" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "pop", author: "chris kempson (http://chriskempson.com)", base00: "#000000", base01: "#202020", base02: "#303030", base03: "#505050", base04: "#b0b0b0", base05: "#d0d0d0", base06: "#e0e0e0", base07: "#ffffff", base08: "#eb008a", base09: "#f29333", base0A: "#f8ca12", base0B: "#37b349", base0C: "#00aabb", base0D: "#0e5a94", base0E: "#b31e8d", base0F: "#7a2d00" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "railscasts", author: "ryan bates (http://railscasts.com)", base00: "#2b2b2b", base01: "#272935", base02: "#3a4055", base03: "#5a647e", base04: "#d4cfc9", base05: "#e6e1dc", base06: "#f4f1ed", base07: "#f9f7f3", base08: "#da4939", base09: "#cc7833", base0A: "#ffc66d", base0B: "#a5c261", base0C: "#519f50", base0D: "#6d9cbe", base0E: "#b6b3eb", base0F: "#bc9458" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "shapeshifter", author: "tyler benziger (http://tybenz.com)", base00: "#000000", base01: "#040404", base02: "#102015", base03: "#343434", base04: "#555555", base05: "#ababab", base06: "#e0e0e0", base07: "#f9f9f9", base08: "#e92f2f", base09: "#e09448", base0A: "#dddd13", base0B: "#0ed839", base0C: "#23edda", base0D: "#3b48e3", base0E: "#f996e2", base0F: "#69542d" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "solarized", author: "ethan schoonover (http://ethanschoonover.com/solarized)", base00: "#002b36", base01: "#073642", base02: "#586e75", base03: "#657b83", base04: "#839496", base05: "#93a1a1", base06: "#eee8d5", base07: "#fdf6e3", base08: "#dc322f", base09: "#cb4b16", base0A: "#b58900", base0B: "#859900", base0C: "#2aa198", base0D: "#268bd2", base0E: "#6c71c4", base0F: "#d33682" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "summerfruit", author: "christopher corley (http://cscorley.github.io/)", base00: "#151515", base01: "#202020", base02: "#303030", base03: "#505050", base04: "#B0B0B0", base05: "#D0D0D0", base06: "#E0E0E0", base07: "#FFFFFF", base08: "#FF0086", base09: "#FD8900", base0A: "#ABA800", base0B: "#00C918", base0C: "#1faaaa", base0D: "#3777E6", base0E: "#AD00A1", base0F: "#cc6633" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "tomorrow", author: "chris kempson (http://chriskempson.com)", base00: "#1d1f21", base01: "#282a2e", base02: "#373b41", base03: "#969896", base04: "#b4b7b4", base05: "#c5c8c6", base06: "#e0e0e0", base07: "#ffffff", base08: "#cc6666", base09: "#de935f", base0A: "#f0c674", base0B: "#b5bd68", base0C: "#8abeb7", base0D: "#81a2be", base0E: "#b294bb", base0F: "#a3685a" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "london tube", author: "jan t. sott", base00: "#231f20", base01: "#1c3f95", base02: "#5a5758", base03: "#737171", base04: "#959ca1", base05: "#d9d8d8", base06: "#e7e7e8", base07: "#ffffff", base08: "#ee2e24", base09: "#f386a1", base0A: "#ffd204", base0B: "#00853e", base0C: "#85cebc", base0D: "#009ddc", base0E: "#98005d", base0F: "#b06110" }, r.exports = n.default;
      }, function(r, n, o) {
        n.__esModule = !0, n.default = { scheme: "twilight", author: "david hart (http://hart-dev.com)", base00: "#1e1e1e", base01: "#323537", base02: "#464b50", base03: "#5f5a60", base04: "#838184", base05: "#a7a7a7", base06: "#c3c3c3", base07: "#ffffff", base08: "#cf6a4c", base09: "#cda869", base0A: "#f9ee98", base0B: "#8f9d6a", base0C: "#afc4db", base0D: "#7587a6", base0E: "#9b859d", base0F: "#9b703f" }, r.exports = n.default;
      }, function(r, n, o) {
        var i = o(33);
        function s(l) {
          var u = Math.round(i(l, 0, 255)).toString(16);
          return u.length == 1 ? "0" + u : u;
        }
        r.exports = function(l) {
          var u = l.length === 4 ? s(255 * l[3]) : "";
          return "#" + s(l[0]) + s(l[1]) + s(l[2]) + u;
        };
      }, function(r, n, o) {
        var i = o(134), s = o(135), l = o(136), u = o(137), c = { "#": s, hsl: function(p) {
          var h = i(p), g = u(h);
          return h.length === 4 && g.push(h[3]), g;
        }, rgb: l };
        function d(p) {
          for (var h in c) if (p.indexOf(h) === 0) return c[h](p);
        }
        d.rgb = l, d.hsl = i, d.hex = s, r.exports = d;
      }, function(r, n, o) {
        var i = o(44), s = o(33);
        function l(u, c) {
          switch (u = parseFloat(u), c) {
            case 0:
              return s(u, 0, 360);
            case 1:
            case 2:
              return s(u, 0, 100);
            case 3:
              return s(u, 0, 1);
          }
        }
        r.exports = function(u) {
          return i(u).map(l);
        };
      }, function(r, n) {
        r.exports = function(o) {
          o.length !== 4 && o.length !== 5 || (o = (function(l) {
            for (var u = "#", c = 1; c < l.length; c++) {
              var d = l.charAt(c);
              u += d + d;
            }
            return u;
          })(o));
          var i = [parseInt(o.substring(1, 3), 16), parseInt(o.substring(3, 5), 16), parseInt(o.substring(5, 7), 16)];
          if (o.length === 9) {
            var s = parseFloat((parseInt(o.substring(7, 9), 16) / 255).toFixed(2));
            i.push(s);
          }
          return i;
        };
      }, function(r, n, o) {
        var i = o(44), s = o(33);
        function l(u, c) {
          return c < 3 ? u.indexOf("%") != -1 ? Math.round(255 * s(parseInt(u, 10), 0, 100) / 100) : s(parseInt(u, 10), 0, 255) : s(parseFloat(u), 0, 1);
        }
        r.exports = function(u) {
          return i(u).map(l);
        };
      }, function(r, n) {
        r.exports = function(o) {
          var i, s, l, u, c, d = o[0] / 360, p = o[1] / 100, h = o[2] / 100;
          if (p == 0) return [c = 255 * h, c, c];
          i = 2 * h - (s = h < 0.5 ? h * (1 + p) : h + p - h * p), u = [0, 0, 0];
          for (var g = 0; g < 3; g++) (l = d + 1 / 3 * -(g - 1)) < 0 && l++, l > 1 && l--, c = 6 * l < 1 ? i + 6 * (s - i) * l : 2 * l < 1 ? s : 3 * l < 2 ? i + (s - i) * (2 / 3 - l) * 6 : i, u[g] = 255 * c;
          return u;
        };
      }, function(r, n, o) {
        (function(i) {
          var s = typeof i == "object" && i && i.Object === Object && i, l = typeof self == "object" && self && self.Object === Object && self, u = s || l || Function("return this")();
          function c(O, P, A) {
            switch (A.length) {
              case 0:
                return O.call(P);
              case 1:
                return O.call(P, A[0]);
              case 2:
                return O.call(P, A[0], A[1]);
              case 3:
                return O.call(P, A[0], A[1], A[2]);
            }
            return O.apply(P, A);
          }
          function d(O, P) {
            for (var A = -1, v = P.length, C = O.length; ++A < v; ) O[C + A] = P[A];
            return O;
          }
          var p = Object.prototype, h = p.hasOwnProperty, g = p.toString, m = u.Symbol, y = p.propertyIsEnumerable, w = m ? m.isConcatSpreadable : void 0, k = Math.max;
          function j(O) {
            return f(O) || (function(P) {
              return (function(A) {
                return /* @__PURE__ */ (function(v) {
                  return !!v && typeof v == "object";
                })(A) && (function(v) {
                  return v != null && (function(C) {
                    return typeof C == "number" && C > -1 && C % 1 == 0 && C <= 9007199254740991;
                  })(v.length) && !(function(C) {
                    var M = (function(D) {
                      var $ = typeof D;
                      return !!D && ($ == "object" || $ == "function");
                    })(C) ? g.call(C) : "";
                    return M == "[object Function]" || M == "[object GeneratorFunction]";
                  })(v);
                })(A);
              })(P) && h.call(P, "callee") && (!y.call(P, "callee") || g.call(P) == "[object Arguments]");
            })(O) || !!(w && O && O[w]);
          }
          var f = Array.isArray, x, E, L, W = (E = function(O) {
            var P = (O = (function(C, M, D, $, H) {
              var F = -1, q = C.length;
              for (D || (D = j), H || (H = []); ++F < q; ) {
                var z = C[F];
                D(z) ? d(H, z) : H[H.length] = z;
              }
              return H;
            })(O)).length, A = P;
            for (x; A--; ) if (typeof O[A] != "function") throw new TypeError("Expected a function");
            return function() {
              for (var v = 0, C = P ? O[v].apply(this, arguments) : arguments[0]; ++v < P; ) C = O[v].call(this, C);
              return C;
            };
          }, L = k(L === void 0 ? E.length - 1 : L, 0), function() {
            for (var O = arguments, P = -1, A = k(O.length - L, 0), v = Array(A); ++P < A; ) v[P] = O[L + P];
            P = -1;
            for (var C = Array(L + 1); ++P < L; ) C[P] = O[P];
            return C[L] = v, c(E, this, C);
          });
          r.exports = W;
        }).call(this, o(43));
      }, function(r, n, o) {
        Object.defineProperty(n, "__esModule", { value: !0 }), n.yuv2rgb = function(i) {
          var s, l, u, c = i[0], d = i[1], p = i[2];
          return s = 1 * c + 0 * d + 1.13983 * p, l = 1 * c + -0.39465 * d + -0.5806 * p, u = 1 * c + 2.02311 * d + 0 * p, s = Math.min(Math.max(0, s), 1), l = Math.min(Math.max(0, l), 1), u = Math.min(Math.max(0, u), 1), [255 * s, 255 * l, 255 * u];
        }, n.rgb2yuv = function(i) {
          var s = i[0] / 255, l = i[1] / 255, u = i[2] / 255;
          return [0.299 * s + 0.587 * l + 0.114 * u, -0.14713 * s + -0.28886 * l + 0.436 * u, 0.615 * s + -0.51499 * l + -0.10001 * u];
        };
      }, function(r, n, o) {
        function i(u, c, d) {
          return c in u ? Object.defineProperty(u, c, { value: d, enumerable: !0, configurable: !0, writable: !0 }) : u[c] = d, u;
        }
        var s = o(141), l = (function() {
          function u() {
            i(this, "_callbacks", void 0), i(this, "_isDispatching", void 0), i(this, "_isHandled", void 0), i(this, "_isPending", void 0), i(this, "_lastID", void 0), i(this, "_pendingPayload", void 0), this._callbacks = {}, this._isDispatching = !1, this._isHandled = {}, this._isPending = {}, this._lastID = 1;
          }
          var c = u.prototype;
          return c.register = function(d) {
            var p = "ID_" + this._lastID++;
            return this._callbacks[p] = d, p;
          }, c.unregister = function(d) {
            this._callbacks[d] || s(!1), delete this._callbacks[d];
          }, c.waitFor = function(d) {
            this._isDispatching || s(!1);
            for (var p = 0; p < d.length; p++) {
              var h = d[p];
              this._isPending[h] ? this._isHandled[h] || s(!1) : (this._callbacks[h] || s(!1), this._invokeCallback(h));
            }
          }, c.dispatch = function(d) {
            this._isDispatching && s(!1), this._startDispatching(d);
            try {
              for (var p in this._callbacks) this._isPending[p] || this._invokeCallback(p);
            } finally {
              this._stopDispatching();
            }
          }, c.isDispatching = function() {
            return this._isDispatching;
          }, c._invokeCallback = function(d) {
            this._isPending[d] = !0, this._callbacks[d](this._pendingPayload), this._isHandled[d] = !0;
          }, c._startDispatching = function(d) {
            for (var p in this._callbacks) this._isPending[p] = !1, this._isHandled[p] = !1;
            this._pendingPayload = d, this._isDispatching = !0;
          }, c._stopDispatching = function() {
            delete this._pendingPayload, this._isDispatching = !1;
          }, u;
        })();
        r.exports = l;
      }, function(r, n, o) {
        r.exports = function(i, s) {
          for (var l = arguments.length, u = new Array(l > 2 ? l - 2 : 0), c = 2; c < l; c++) u[c - 2] = arguments[c];
          if (!i) {
            var d;
            if (s === void 0) d = new Error("Minified exception occurred; use the non-minified dev environment for the full error message and additional helpful warnings.");
            else {
              var p = 0;
              (d = new Error(s.replace(/%s/g, (function() {
                return String(u[p++]);
              })))).name = "Invariant Violation";
            }
            throw d.framesToPop = 1, d;
          }
        };
      }, function(r, n, o) {
        function i(I, N, S) {
          return N in I ? Object.defineProperty(I, N, { value: S, enumerable: !0, configurable: !0, writable: !0 }) : I[N] = S, I;
        }
        function s(I, N) {
          var S = Object.keys(I);
          if (Object.getOwnPropertySymbols) {
            var R = Object.getOwnPropertySymbols(I);
            N && (R = R.filter((function(_) {
              return Object.getOwnPropertyDescriptor(I, _).enumerable;
            }))), S.push.apply(S, R);
          }
          return S;
        }
        function l(I) {
          for (var N = 1; N < arguments.length; N++) {
            var S = arguments[N] != null ? arguments[N] : {};
            N % 2 ? s(Object(S), !0).forEach((function(R) {
              i(I, R, S[R]);
            })) : Object.getOwnPropertyDescriptors ? Object.defineProperties(I, Object.getOwnPropertyDescriptors(S)) : s(Object(S)).forEach((function(R) {
              Object.defineProperty(I, R, Object.getOwnPropertyDescriptor(S, R));
            }));
          }
          return I;
        }
        function u(I, N) {
          if (!(I instanceof N)) throw new TypeError("Cannot call a class as a function");
        }
        function c(I, N) {
          for (var S = 0; S < N.length; S++) {
            var R = N[S];
            R.enumerable = R.enumerable || !1, R.configurable = !0, "value" in R && (R.writable = !0), Object.defineProperty(I, R.key, R);
          }
        }
        function d(I, N, S) {
          return N && c(I.prototype, N), S && c(I, S), I;
        }
        function p(I, N) {
          return (p = Object.setPrototypeOf || function(S, R) {
            return S.__proto__ = R, S;
          })(I, N);
        }
        function h(I, N) {
          if (typeof N != "function" && N !== null) throw new TypeError("Super expression must either be null or a function");
          I.prototype = Object.create(N && N.prototype, { constructor: { value: I, writable: !0, configurable: !0 } }), N && p(I, N);
        }
        function g(I) {
          return (g = Object.setPrototypeOf ? Object.getPrototypeOf : function(N) {
            return N.__proto__ || Object.getPrototypeOf(N);
          })(I);
        }
        function m(I) {
          return (m = typeof Symbol == "function" && typeof Symbol.iterator == "symbol" ? function(N) {
            return typeof N;
          } : function(N) {
            return N && typeof Symbol == "function" && N.constructor === Symbol && N !== Symbol.prototype ? "symbol" : typeof N;
          })(I);
        }
        function y(I) {
          if (I === void 0) throw new ReferenceError("this hasn't been initialised - super() hasn't been called");
          return I;
        }
        function w(I, N) {
          return !N || m(N) !== "object" && typeof N != "function" ? y(I) : N;
        }
        function k(I) {
          var N = (function() {
            if (typeof Reflect > "u" || !Reflect.construct || Reflect.construct.sham) return !1;
            if (typeof Proxy == "function") return !0;
            try {
              return Date.prototype.toString.call(Reflect.construct(Date, [], (function() {
              }))), !0;
            } catch {
              return !1;
            }
          })();
          return function() {
            var S, R = g(I);
            if (N) {
              var _ = g(this).constructor;
              S = Reflect.construct(R, arguments, _);
            } else S = R.apply(this, arguments);
            return w(this, S);
          };
        }
        o.r(n);
        var j = o(0), f = o.n(j);
        function x() {
          var I = this.constructor.getDerivedStateFromProps(this.props, this.state);
          I != null && this.setState(I);
        }
        function E(I) {
          this.setState((function(N) {
            var S = this.constructor.getDerivedStateFromProps(I, N);
            return S ?? null;
          }).bind(this));
        }
        function L(I, N) {
          try {
            var S = this.props, R = this.state;
            this.props = I, this.state = N, this.__reactInternalSnapshotFlag = !0, this.__reactInternalSnapshot = this.getSnapshotBeforeUpdate(S, R);
          } finally {
            this.props = S, this.state = R;
          }
        }
        function W(I) {
          var N = I.prototype;
          if (!N || !N.isReactComponent) throw new Error("Can only polyfill class components");
          if (typeof I.getDerivedStateFromProps != "function" && typeof N.getSnapshotBeforeUpdate != "function") return I;
          var S = null, R = null, _ = null;
          if (typeof N.componentWillMount == "function" ? S = "componentWillMount" : typeof N.UNSAFE_componentWillMount == "function" && (S = "UNSAFE_componentWillMount"), typeof N.componentWillReceiveProps == "function" ? R = "componentWillReceiveProps" : typeof N.UNSAFE_componentWillReceiveProps == "function" && (R = "UNSAFE_componentWillReceiveProps"), typeof N.componentWillUpdate == "function" ? _ = "componentWillUpdate" : typeof N.UNSAFE_componentWillUpdate == "function" && (_ = "UNSAFE_componentWillUpdate"), S !== null || R !== null || _ !== null) {
            var G = I.displayName || I.name, re = typeof I.getDerivedStateFromProps == "function" ? "getDerivedStateFromProps()" : "getSnapshotBeforeUpdate()";
            throw Error(`Unsafe legacy lifecycles will not be called for components using new component APIs.

` + G + " uses " + re + " but also contains the following legacy lifecycles:" + (S !== null ? `
  ` + S : "") + (R !== null ? `
  ` + R : "") + (_ !== null ? `
  ` + _ : "") + `

The above lifecycles should be removed. Learn more about this warning here:
https://fb.me/react-async-component-lifecycle-hooks`);
          }
          if (typeof I.getDerivedStateFromProps == "function" && (N.componentWillMount = x, N.componentWillReceiveProps = E), typeof N.getSnapshotBeforeUpdate == "function") {
            if (typeof N.componentDidUpdate != "function") throw new Error("Cannot polyfill getSnapshotBeforeUpdate() for components that do not define componentDidUpdate() on the prototype");
            N.componentWillUpdate = L;
            var ee = N.componentDidUpdate;
            N.componentDidUpdate = function(K, se, Oe) {
              var Xe = this.__reactInternalSnapshotFlag ? this.__reactInternalSnapshot : Oe;
              ee.call(this, K, se, Xe);
            };
          }
          return I;
        }
        function O(I, N) {
          if (I == null) return {};
          var S, R, _ = (function(re, ee) {
            if (re == null) return {};
            var K, se, Oe = {}, Xe = Object.keys(re);
            for (se = 0; se < Xe.length; se++) K = Xe[se], ee.indexOf(K) >= 0 || (Oe[K] = re[K]);
            return Oe;
          })(I, N);
          if (Object.getOwnPropertySymbols) {
            var G = Object.getOwnPropertySymbols(I);
            for (R = 0; R < G.length; R++) S = G[R], N.indexOf(S) >= 0 || Object.prototype.propertyIsEnumerable.call(I, S) && (_[S] = I[S]);
          }
          return _;
        }
        function P(I) {
          var N = (function(S) {
            return {}.toString.call(S).match(/\s([a-zA-Z]+)/)[1].toLowerCase();
          })(I);
          return N === "number" && (N = isNaN(I) ? "nan" : (0 | I) != I ? "float" : "integer"), N;
        }
        x.__suppressDeprecationWarning = !0, E.__suppressDeprecationWarning = !0, L.__suppressDeprecationWarning = !0;
        var A = { scheme: "rjv-default", author: "mac gainor", base00: "rgba(0, 0, 0, 0)", base01: "rgb(245, 245, 245)", base02: "rgb(235, 235, 235)", base03: "#93a1a1", base04: "rgba(0, 0, 0, 0.3)", base05: "#586e75", base06: "#073642", base07: "#002b36", base08: "#d33682", base09: "#cb4b16", base0A: "#dc322f", base0B: "#859900", base0C: "#6c71c4", base0D: "#586e75", base0E: "#2aa198", base0F: "#268bd2" }, v = { scheme: "rjv-grey", author: "mac gainor", base00: "rgba(1, 1, 1, 0)", base01: "rgba(1, 1, 1, 0.1)", base02: "rgba(0, 0, 0, 0.2)", base03: "rgba(1, 1, 1, 0.3)", base04: "rgba(0, 0, 0, 0.4)", base05: "rgba(1, 1, 1, 0.5)", base06: "rgba(1, 1, 1, 0.6)", base07: "rgba(1, 1, 1, 0.7)", base08: "rgba(1, 1, 1, 0.8)", base09: "rgba(1, 1, 1, 0.8)", base0A: "rgba(1, 1, 1, 0.8)", base0B: "rgba(1, 1, 1, 0.8)", base0C: "rgba(1, 1, 1, 0.8)", base0D: "rgba(1, 1, 1, 0.8)", base0E: "rgba(1, 1, 1, 0.8)", base0F: "rgba(1, 1, 1, 0.8)" }, C = { globalFontFamily: "monospace", globalCursor: "default", braceFontWeight: "bold", braceCursor: "pointer", ellipsisFontSize: "18px", ellipsisLineHeight: "10px", ellipsisCursor: "pointer", keyMargin: "0px 5px", keyLetterSpacing: "0.5px", keyFontStyle: "none", keyVerticalAlign: "top", keyOpacity: "0.85", keyOpacityHover: "1", keyValPaddingTop: "3px", keyValPaddingBottom: "3px", keyValPaddingRight: "5px", keyValBorderLeft: "1px solid", keyValBorderHover: "2px solid", pushedContentMarginLeft: "6px", variableValuePaddingRight: "6px", nullFontSize: "11px", nullFontWeight: "bold", nullPadding: "1px 2px", nullBorderRadius: "3px", nanFontSize: "11px", nanFontWeight: "bold", nanPadding: "1px 2px", nanBorderRadius: "3px", undefinedFontSize: "11px", undefinedPadding: "1px 2px", undefinedBorderRadius: "3px", dataTypeFontSize: "11px", dataTypeMarginRight: "4px", datatypeOpacity: "0.8", objectSizeBorderRadius: "3px", objectSizeFontStyle: "italic", objectSizeMargin: "0px 6px 0px 0px", clipboardCursor: "pointer", clipboardCheckMarginLeft: "-12px", metaDataPadding: "0px 0px 0px 10px", arrayGroupMetaPadding: "0px 0px 0px 4px", iconContainerWidth: "17px", tooltipPadding: "4px", editInputMinWidth: "130px", editInputBorderRadius: "2px", editInputPadding: "5px", editInputMarginRight: "4px", editInputFontFamily: "monospace", iconCursor: "pointer", iconFontSize: "15px", iconPaddingRight: "1px", dateValueMarginLeft: "2px", iconMarginRight: "3px", detectedRowPaddingTop: "3px", addKeyCoverBackground: "rgba(255, 255, 255, 0.3)", addKeyCoverPosition: "absolute", addKeyCoverPositionPx: "0px", addKeyModalWidth: "200px", addKeyModalMargin: "auto", addKeyModalPadding: "10px", addKeyModalRadius: "3px" }, M = o(45), D = function(I) {
          var N = (function(S) {
            return { backgroundColor: S.base00, ellipsisColor: S.base09, braceColor: S.base07, expandedIcon: S.base0D, collapsedIcon: S.base0E, keyColor: S.base07, arrayKeyColor: S.base0C, objectSize: S.base04, copyToClipboard: S.base0F, copyToClipboardCheck: S.base0D, objectBorder: S.base02, dataTypes: { boolean: S.base0E, date: S.base0D, float: S.base0B, function: S.base0D, integer: S.base0F, string: S.base09, nan: S.base08, null: S.base0A, undefined: S.base05, regexp: S.base0A, background: S.base02 }, editVariable: { editIcon: S.base0E, cancelIcon: S.base09, removeIcon: S.base09, addIcon: S.base0E, checkIcon: S.base0E, background: S.base01, color: S.base0A, border: S.base07 }, addKeyModal: { background: S.base05, border: S.base04, color: S.base0A, labelColor: S.base01 }, validationFailure: { background: S.base09, iconColor: S.base01, fontColor: S.base01 } };
          })(I);
          return { "app-container": { fontFamily: C.globalFontFamily, cursor: C.globalCursor, backgroundColor: N.backgroundColor, position: "relative" }, ellipsis: { display: "inline-block", color: N.ellipsisColor, fontSize: C.ellipsisFontSize, lineHeight: C.ellipsisLineHeight, cursor: C.ellipsisCursor }, "brace-row": { display: "inline-block", cursor: "pointer" }, brace: { display: "inline-block", cursor: C.braceCursor, fontWeight: C.braceFontWeight, color: N.braceColor }, "expanded-icon": { color: N.expandedIcon }, "collapsed-icon": { color: N.collapsedIcon }, colon: { display: "inline-block", margin: C.keyMargin, color: N.keyColor, verticalAlign: "top" }, objectKeyVal: function(S, R) {
            return { style: l({ paddingTop: C.keyValPaddingTop, paddingRight: C.keyValPaddingRight, paddingBottom: C.keyValPaddingBottom, borderLeft: C.keyValBorderLeft + " " + N.objectBorder, ":hover": { paddingLeft: R.paddingLeft - 1 + "px", borderLeft: C.keyValBorderHover + " " + N.objectBorder } }, R) };
          }, "object-key-val-no-border": { padding: C.keyValPadding }, "pushed-content": { marginLeft: C.pushedContentMarginLeft }, variableValue: function(S, R) {
            return { style: l({ display: "inline-block", paddingRight: C.variableValuePaddingRight, position: "relative" }, R) };
          }, "object-name": { display: "inline-block", color: N.keyColor, letterSpacing: C.keyLetterSpacing, fontStyle: C.keyFontStyle, verticalAlign: C.keyVerticalAlign, opacity: C.keyOpacity, ":hover": { opacity: C.keyOpacityHover } }, "array-key": { display: "inline-block", color: N.arrayKeyColor, letterSpacing: C.keyLetterSpacing, fontStyle: C.keyFontStyle, verticalAlign: C.keyVerticalAlign, opacity: C.keyOpacity, ":hover": { opacity: C.keyOpacityHover } }, "object-size": { color: N.objectSize, borderRadius: C.objectSizeBorderRadius, fontStyle: C.objectSizeFontStyle, margin: C.objectSizeMargin, cursor: "default" }, "data-type-label": { fontSize: C.dataTypeFontSize, marginRight: C.dataTypeMarginRight, opacity: C.datatypeOpacity }, boolean: { display: "inline-block", color: N.dataTypes.boolean }, date: { display: "inline-block", color: N.dataTypes.date }, "date-value": { marginLeft: C.dateValueMarginLeft }, float: { display: "inline-block", color: N.dataTypes.float }, function: { display: "inline-block", color: N.dataTypes.function, cursor: "pointer", whiteSpace: "pre-line" }, "function-value": { fontStyle: "italic" }, integer: { display: "inline-block", color: N.dataTypes.integer }, string: { display: "inline-block", color: N.dataTypes.string }, nan: { display: "inline-block", color: N.dataTypes.nan, fontSize: C.nanFontSize, fontWeight: C.nanFontWeight, backgroundColor: N.dataTypes.background, padding: C.nanPadding, borderRadius: C.nanBorderRadius }, null: { display: "inline-block", color: N.dataTypes.null, fontSize: C.nullFontSize, fontWeight: C.nullFontWeight, backgroundColor: N.dataTypes.background, padding: C.nullPadding, borderRadius: C.nullBorderRadius }, undefined: { display: "inline-block", color: N.dataTypes.undefined, fontSize: C.undefinedFontSize, padding: C.undefinedPadding, borderRadius: C.undefinedBorderRadius, backgroundColor: N.dataTypes.background }, regexp: { display: "inline-block", color: N.dataTypes.regexp }, "copy-to-clipboard": { cursor: C.clipboardCursor }, "copy-icon": { color: N.copyToClipboard, fontSize: C.iconFontSize, marginRight: C.iconMarginRight, verticalAlign: "top" }, "copy-icon-copied": { color: N.copyToClipboardCheck, marginLeft: C.clipboardCheckMarginLeft }, "array-group-meta-data": { display: "inline-block", padding: C.arrayGroupMetaPadding }, "object-meta-data": { display: "inline-block", padding: C.metaDataPadding }, "icon-container": { display: "inline-block", width: C.iconContainerWidth }, tooltip: { padding: C.tooltipPadding }, removeVarIcon: { verticalAlign: "top", display: "inline-block", color: N.editVariable.removeIcon, cursor: C.iconCursor, fontSize: C.iconFontSize, marginRight: C.iconMarginRight }, addVarIcon: { verticalAlign: "top", display: "inline-block", color: N.editVariable.addIcon, cursor: C.iconCursor, fontSize: C.iconFontSize, marginRight: C.iconMarginRight }, editVarIcon: { verticalAlign: "top", display: "inline-block", color: N.editVariable.editIcon, cursor: C.iconCursor, fontSize: C.iconFontSize, marginRight: C.iconMarginRight }, "edit-icon-container": { display: "inline-block", verticalAlign: "top" }, "check-icon": { display: "inline-block", cursor: C.iconCursor, color: N.editVariable.checkIcon, fontSize: C.iconFontSize, paddingRight: C.iconPaddingRight }, "cancel-icon": { display: "inline-block", cursor: C.iconCursor, color: N.editVariable.cancelIcon, fontSize: C.iconFontSize, paddingRight: C.iconPaddingRight }, "edit-input": { display: "inline-block", minWidth: C.editInputMinWidth, borderRadius: C.editInputBorderRadius, backgroundColor: N.editVariable.background, color: N.editVariable.color, padding: C.editInputPadding, marginRight: C.editInputMarginRight, fontFamily: C.editInputFontFamily }, "detected-row": { paddingTop: C.detectedRowPaddingTop }, "key-modal-request": { position: C.addKeyCoverPosition, top: C.addKeyCoverPositionPx, left: C.addKeyCoverPositionPx, right: C.addKeyCoverPositionPx, bottom: C.addKeyCoverPositionPx, backgroundColor: C.addKeyCoverBackground }, "key-modal": { width: C.addKeyModalWidth, backgroundColor: N.addKeyModal.background, marginLeft: C.addKeyModalMargin, marginRight: C.addKeyModalMargin, padding: C.addKeyModalPadding, borderRadius: C.addKeyModalRadius, marginTop: "15px", position: "relative" }, "key-modal-label": { color: N.addKeyModal.labelColor, marginLeft: "2px", marginBottom: "5px", fontSize: "11px" }, "key-modal-input-container": { overflow: "hidden" }, "key-modal-input": { width: "100%", padding: "3px 6px", fontFamily: "monospace", color: N.addKeyModal.color, border: "none", boxSizing: "border-box", borderRadius: "2px" }, "key-modal-cancel": { backgroundColor: N.editVariable.removeIcon, position: "absolute", top: "0px", right: "0px", borderRadius: "0px 3px 0px 3px", cursor: "pointer" }, "key-modal-cancel-icon": { color: N.addKeyModal.labelColor, fontSize: C.iconFontSize, transform: "rotate(45deg)" }, "key-modal-submit": { color: N.editVariable.addIcon, fontSize: C.iconFontSize, position: "absolute", right: "2px", top: "3px", cursor: "pointer" }, "function-ellipsis": { display: "inline-block", color: N.ellipsisColor, fontSize: C.ellipsisFontSize, lineHeight: C.ellipsisLineHeight, cursor: C.ellipsisCursor }, "validation-failure": { float: "right", padding: "3px 6px", borderRadius: "2px", cursor: "pointer", color: N.validationFailure.fontColor, backgroundColor: N.validationFailure.background }, "validation-failure-label": { marginRight: "6px" }, "validation-failure-clear": { position: "relative", verticalAlign: "top", cursor: "pointer", color: N.validationFailure.iconColor, fontSize: C.iconFontSize, transform: "rotate(45deg)" } };
        };
        function $(I, N, S) {
          return I || console.error("theme has not been set"), (function(R) {
            var _ = A;
            return R !== !1 && R !== "none" || (_ = v), Object(M.createStyling)(D, { defaultBase16: _ })(R);
          })(I)(N, S);
        }
        var H = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = (R.rjvId, R.type_name), G = R.displayDataTypes, re = R.theme;
            return G ? f.a.createElement("span", Object.assign({ className: "data-type-label" }, $(re, "data-type-label")), _) : null;
          } }]), S;
        })(f.a.PureComponent), F = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props;
            return f.a.createElement("div", $(R.theme, "boolean"), f.a.createElement(H, Object.assign({ type_name: "bool" }, R)), R.value ? "true" : "false");
          } }]), S;
        })(f.a.PureComponent), q = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props;
            return f.a.createElement("div", $(R.theme, "date"), f.a.createElement(H, Object.assign({ type_name: "date" }, R)), f.a.createElement("span", Object.assign({ className: "date-value" }, $(R.theme, "date-value")), R.value.toLocaleTimeString("en-us", { weekday: "short", year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })));
          } }]), S;
        })(f.a.PureComponent), z = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props;
            return f.a.createElement("div", $(R.theme, "float"), f.a.createElement(H, Object.assign({ type_name: "float" }, R)), this.props.value);
          } }]), S;
        })(f.a.PureComponent);
        function Z(I, N) {
          (N == null || N > I.length) && (N = I.length);
          for (var S = 0, R = new Array(N); S < N; S++) R[S] = I[S];
          return R;
        }
        function Q(I, N) {
          if (I) {
            if (typeof I == "string") return Z(I, N);
            var S = Object.prototype.toString.call(I).slice(8, -1);
            return S === "Object" && I.constructor && (S = I.constructor.name), S === "Map" || S === "Set" ? Array.from(I) : S === "Arguments" || /^(?:Ui|I)nt(?:8|16|32)(?:Clamped)?Array$/.test(S) ? Z(I, N) : void 0;
          }
        }
        function ue(I, N) {
          var S;
          if (typeof Symbol > "u" || I[Symbol.iterator] == null) {
            if (Array.isArray(I) || (S = Q(I)) || N) {
              S && (I = S);
              var R = 0, _ = function() {
              };
              return { s: _, n: function() {
                return R >= I.length ? { done: !0 } : { done: !1, value: I[R++] };
              }, e: function(K) {
                throw K;
              }, f: _ };
            }
            throw new TypeError(`Invalid attempt to iterate non-iterable instance.
In order to be iterable, non-array objects must have a [Symbol.iterator]() method.`);
          }
          var G, re = !0, ee = !1;
          return { s: function() {
            S = I[Symbol.iterator]();
          }, n: function() {
            var K = S.next();
            return re = K.done, K;
          }, e: function(K) {
            ee = !0, G = K;
          }, f: function() {
            try {
              re || S.return == null || S.return();
            } finally {
              if (ee) throw G;
            }
          } };
        }
        function de(I) {
          return (function(N) {
            if (Array.isArray(N)) return Z(N);
          })(I) || (function(N) {
            if (typeof Symbol < "u" && Symbol.iterator in Object(N)) return Array.from(N);
          })(I) || Q(I) || (function() {
            throw new TypeError(`Invalid attempt to spread non-iterable instance.
In order to be iterable, non-array objects must have a [Symbol.iterator]() method.`);
          })();
        }
        var Se = o(46), fe = new (o(47)).Dispatcher(), ye = new ((function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            var R;
            u(this, S);
            for (var _ = arguments.length, G = new Array(_), re = 0; re < _; re++) G[re] = arguments[re];
            return (R = N.call.apply(N, [this].concat(G))).objects = {}, R.set = function(ee, K, se, Oe) {
              R.objects[ee] === void 0 && (R.objects[ee] = {}), R.objects[ee][K] === void 0 && (R.objects[ee][K] = {}), R.objects[ee][K][se] = Oe;
            }, R.get = function(ee, K, se, Oe) {
              return R.objects[ee] === void 0 || R.objects[ee][K] === void 0 || R.objects[ee][K][se] == null ? Oe : R.objects[ee][K][se];
            }, R.handleAction = function(ee) {
              var K = ee.rjvId, se = ee.data;
              switch (ee.name) {
                case "RESET":
                  R.emit("reset-" + K);
                  break;
                case "VARIABLE_UPDATED":
                  ee.data.updated_src = R.updateSrc(K, se), R.set(K, "action", "variable-update", l(l({}, se), {}, { type: "variable-edited" })), R.emit("variable-update-" + K);
                  break;
                case "VARIABLE_REMOVED":
                  ee.data.updated_src = R.updateSrc(K, se), R.set(K, "action", "variable-update", l(l({}, se), {}, { type: "variable-removed" })), R.emit("variable-update-" + K);
                  break;
                case "VARIABLE_ADDED":
                  ee.data.updated_src = R.updateSrc(K, se), R.set(K, "action", "variable-update", l(l({}, se), {}, { type: "variable-added" })), R.emit("variable-update-" + K);
                  break;
                case "ADD_VARIABLE_KEY_REQUEST":
                  R.set(K, "action", "new-key-request", se), R.emit("add-key-request-" + K);
              }
            }, R.updateSrc = function(ee, K) {
              var se = K.name, Oe = K.namespace, Xe = K.new_value, nt = (K.existing_value, K.variable_removed);
              Oe.shift();
              var lt, Ge = R.get(ee, "global", "src"), ct = R.deepCopy(Ge, de(Oe)), yt = ct, Be = ue(Oe);
              try {
                for (Be.s(); !(lt = Be.n()).done; )
                  yt = yt[lt.value];
              } catch ($t) {
                Be.e($t);
              } finally {
                Be.f();
              }
              return nt ? P(yt) == "array" ? yt.splice(se, 1) : delete yt[se] : se !== null ? yt[se] = Xe : ct = Xe, R.set(ee, "global", "src", ct), ct;
            }, R.deepCopy = function(ee, K) {
              var se, Oe = P(ee), Xe = K.shift();
              return Oe == "array" ? se = de(ee) : Oe == "object" && (se = l({}, ee)), Xe !== void 0 && (se[Xe] = R.deepCopy(ee[Xe], K)), se;
            }, R;
          }
          return S;
        })(Se.EventEmitter))();
        fe.register(ye.handleAction.bind(ye));
        var be = ye, we = (function(I) {
          h(S, I);
          var N = k(S);
          function S(R) {
            var _;
            return u(this, S), (_ = N.call(this, R)).toggleCollapsed = function() {
              _.setState({ collapsed: !_.state.collapsed }, (function() {
                be.set(_.props.rjvId, _.props.namespace, "collapsed", _.state.collapsed);
              }));
            }, _.getFunctionDisplay = function(G) {
              var re = y(_).props;
              return G ? f.a.createElement("span", null, _.props.value.toString().slice(9, -1).replace(/\{[\s\S]+/, ""), f.a.createElement("span", { className: "function-collapsed", style: { fontWeight: "bold" } }, f.a.createElement("span", null, "{"), f.a.createElement("span", $(re.theme, "ellipsis"), "..."), f.a.createElement("span", null, "}"))) : _.props.value.toString().slice(9, -1);
            }, _.state = { collapsed: be.get(R.rjvId, R.namespace, "collapsed", !0) }, _;
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = this.state.collapsed;
            return f.a.createElement("div", $(R.theme, "function"), f.a.createElement(H, Object.assign({ type_name: "function" }, R)), f.a.createElement("span", Object.assign({}, $(R.theme, "function-value"), { className: "rjv-function-container", onClick: this.toggleCollapsed }), this.getFunctionDisplay(_)));
          } }]), S;
        })(f.a.PureComponent), Ee = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            return f.a.createElement("div", $(this.props.theme, "nan"), "NaN");
          } }]), S;
        })(f.a.PureComponent), ge = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            return f.a.createElement("div", $(this.props.theme, "null"), "NULL");
          } }]), S;
        })(f.a.PureComponent), ke = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props;
            return f.a.createElement("div", $(R.theme, "integer"), f.a.createElement(H, Object.assign({ type_name: "int" }, R)), this.props.value);
          } }]), S;
        })(f.a.PureComponent), je = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props;
            return f.a.createElement("div", $(R.theme, "regexp"), f.a.createElement(H, Object.assign({ type_name: "regexp" }, R)), this.props.value.toString());
          } }]), S;
        })(f.a.PureComponent), Te = (function(I) {
          h(S, I);
          var N = k(S);
          function S(R) {
            var _;
            return u(this, S), (_ = N.call(this, R)).toggleCollapsed = function() {
              _.setState({ collapsed: !_.state.collapsed }, (function() {
                be.set(_.props.rjvId, _.props.namespace, "collapsed", _.state.collapsed);
              }));
            }, _.state = { collapsed: be.get(R.rjvId, R.namespace, "collapsed", !0) }, _;
          }
          return d(S, [{ key: "render", value: function() {
            this.state.collapsed;
            var R = this.props, _ = R.collapseStringsAfterLength, G = R.theme, re = R.value, ee = { style: { cursor: "default" } };
            return P(_) === "integer" && re.length > _ && (ee.style.cursor = "pointer", this.state.collapsed && (re = f.a.createElement("span", null, re.substring(0, _), f.a.createElement("span", $(G, "ellipsis"), " ...")))), f.a.createElement("div", $(G, "string"), f.a.createElement(H, Object.assign({ type_name: "string" }, R)), f.a.createElement("span", Object.assign({ className: "string-value" }, ee, { onClick: this.toggleCollapsed }), '"', re, '"'));
          } }]), S;
        })(f.a.PureComponent), He = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            return f.a.createElement("div", $(this.props.theme, "undefined"), "undefined");
          } }]), S;
        })(f.a.PureComponent);
        function _e() {
          return (_e = Object.assign || function(I) {
            for (var N = 1; N < arguments.length; N++) {
              var S = arguments[N];
              for (var R in S) Object.prototype.hasOwnProperty.call(S, R) && (I[R] = S[R]);
            }
            return I;
          }).apply(this, arguments);
        }
        var Fe = j.useLayoutEffect, We = function(I) {
          var N = Object(j.useRef)(I);
          return Fe((function() {
            N.current = I;
          })), N;
        }, Ue = function(I, N) {
          typeof I != "function" ? I.current = N : I(N);
        }, tt = function(I, N) {
          var S = Object(j.useRef)();
          return Object(j.useCallback)((function(R) {
            I.current = R, S.current && Ue(S.current, null), S.current = N, N && Ue(N, R);
          }), [N]);
        }, Qe = { "min-height": "0", "max-height": "none", height: "0", visibility: "hidden", overflow: "hidden", position: "absolute", "z-index": "-1000", top: "0", right: "0" }, Ve = function(I) {
          Object.keys(Qe).forEach((function(N) {
            I.style.setProperty(N, Qe[N], "important");
          }));
        }, ce = null, pe = function() {
        }, X = ["borderBottomWidth", "borderLeftWidth", "borderRightWidth", "borderTopWidth", "boxSizing", "fontFamily", "fontSize", "fontStyle", "fontWeight", "letterSpacing", "lineHeight", "paddingBottom", "paddingLeft", "paddingRight", "paddingTop", "tabSize", "textIndent", "textRendering", "textTransform", "width"], V = !!document.documentElement.currentStyle, U = function(I, N) {
          var S = I.cacheMeasurements, R = I.maxRows, _ = I.minRows, G = I.onChange, re = G === void 0 ? pe : G, ee = I.onHeightChange, K = ee === void 0 ? pe : ee, se = (function(Be, $t) {
            if (Be == null) return {};
            var _t, Yt, Yr = {}, Xt = Object.keys(Be);
            for (Yt = 0; Yt < Xt.length; Yt++) _t = Xt[Yt], $t.indexOf(_t) >= 0 || (Yr[_t] = Be[_t]);
            return Yr;
          })(I, ["cacheMeasurements", "maxRows", "minRows", "onChange", "onHeightChange"]), Oe, Xe = se.value !== void 0, nt = Object(j.useRef)(null), lt = tt(nt, N), Ge = Object(j.useRef)(0), ct = Object(j.useRef)(), yt = function() {
            var Be = nt.current, $t = S && ct.current ? ct.current : (function(Xt) {
              var Xr = window.getComputedStyle(Xt);
              if (Xr === null) return null;
              var pr, Tt = (pr = Xr, X.reduce((function(Pr, hr) {
                return Pr[hr] = pr[hr], Pr;
              }), {})), Zt = Tt.boxSizing;
              return Zt === "" ? null : (V && Zt === "border-box" && (Tt.width = parseFloat(Tt.width) + parseFloat(Tt.borderRightWidth) + parseFloat(Tt.borderLeftWidth) + parseFloat(Tt.paddingRight) + parseFloat(Tt.paddingLeft) + "px"), { sizingStyle: Tt, paddingSize: parseFloat(Tt.paddingBottom) + parseFloat(Tt.paddingTop), borderSize: parseFloat(Tt.borderBottomWidth) + parseFloat(Tt.borderTopWidth) });
            })(Be);
            if ($t) {
              ct.current = $t;
              var _t = (function(Xt, Xr, pr, Tt) {
                pr === void 0 && (pr = 1), Tt === void 0 && (Tt = 1 / 0), ce || ((ce = document.createElement("textarea")).setAttribute("tab-index", "-1"), ce.setAttribute("aria-hidden", "true"), Ve(ce)), ce.parentNode === null && document.body.appendChild(ce);
                var Zt = Xt.paddingSize, Pr = Xt.borderSize, hr = Xt.sizingStyle, da = hr.boxSizing;
                Object.keys(hr).forEach((function(Zr) {
                  var mr = Zr;
                  ce.style[mr] = hr[mr];
                })), Ve(ce), ce.value = Xr;
                var Jr = (function(Zr, mr) {
                  var fa = Zr.scrollHeight;
                  return mr.sizingStyle.boxSizing === "border-box" ? fa + mr.borderSize : fa - mr.paddingSize;
                })(ce, Xt);
                ce.value = "x";
                var On = ce.scrollHeight - Zt, _n = On * pr;
                da === "border-box" && (_n = _n + Zt + Pr), Jr = Math.max(_n, Jr);
                var Pn = On * Tt;
                return da === "border-box" && (Pn = Pn + Zt + Pr), [Jr = Math.min(Pn, Jr), On];
              })($t, Be.value || Be.placeholder || "x", _, R), Yt = _t[0], Yr = _t[1];
              Ge.current !== Yt && (Ge.current = Yt, Be.style.setProperty("height", Yt + "px", "important"), K(Yt, { rowHeight: Yr }));
            }
          };
          return Object(j.useLayoutEffect)(yt), Oe = We(yt), Object(j.useLayoutEffect)((function() {
            var Be = function($t) {
              Oe.current($t);
            };
            return window.addEventListener("resize", Be), function() {
              window.removeEventListener("resize", Be);
            };
          }), []), Object(j.createElement)("textarea", _e({}, se, { onChange: function(Be) {
            Xe || yt(), re(Be);
          }, ref: lt }));
        }, te = Object(j.forwardRef)(U);
        function oe(I) {
          I = I.trim();
          try {
            if ((I = JSON.stringify(JSON.parse(I)))[0] === "[") return xe("array", JSON.parse(I));
            if (I[0] === "{") return xe("object", JSON.parse(I));
            if (I.match(/\-?\d+\.\d+/) && I.match(/\-?\d+\.\d+/)[0] === I) return xe("float", parseFloat(I));
            if (I.match(/\-?\d+e-\d+/) && I.match(/\-?\d+e-\d+/)[0] === I) return xe("float", Number(I));
            if (I.match(/\-?\d+/) && I.match(/\-?\d+/)[0] === I) return xe("integer", parseInt(I));
            if (I.match(/\-?\d+e\+\d+/) && I.match(/\-?\d+e\+\d+/)[0] === I) return xe("integer", Number(I));
          } catch {
          }
          switch (I = I.toLowerCase()) {
            case "undefined":
              return xe("undefined", void 0);
            case "nan":
              return xe("nan", NaN);
            case "null":
              return xe("null", null);
            case "true":
              return xe("boolean", !0);
            case "false":
              return xe("boolean", !1);
            default:
              if (I = Date.parse(I)) return xe("date", new Date(I));
          }
          return xe(!1, null);
        }
        function xe(I, N) {
          return { type: I, value: N };
        }
        var Me = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]);
            return f.a.createElement("span", G, f.a.createElement("svg", Object.assign({}, Le(_), { viewBox: "0 0 24 24", fill: "currentColor", preserveAspectRatio: "xMidYMid meet" }), f.a.createElement("path", { d: "M12,20C7.59,20 4,16.41 4,12C4,7.59 7.59,4 12,4C16.41,4 20,7.59 20,12C20,16.41 16.41,20 12,20M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2M7,13H17V11H7" })));
          } }]), S;
        })(f.a.PureComponent), Ce = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]);
            return f.a.createElement("span", G, f.a.createElement("svg", Object.assign({}, Le(_), { viewBox: "0 0 24 24", fill: "currentColor", preserveAspectRatio: "xMidYMid meet" }), f.a.createElement("path", { d: "M12,20C7.59,20 4,16.41 4,12C4,7.59 7.59,4 12,4C16.41,4 20,7.59 20,12C20,16.41 16.41,20 12,20M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2M13,7H11V11H7V13H11V17H13V13H17V11H13V7Z" })));
          } }]), S;
        })(f.a.PureComponent), me = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]), re = Le(_).style;
            return f.a.createElement("span", G, f.a.createElement("svg", { fill: re.color, width: re.height, height: re.width, style: re, viewBox: "0 0 1792 1792" }, f.a.createElement("path", { d: "M1344 800v64q0 14-9 23t-23 9h-832q-14 0-23-9t-9-23v-64q0-14 9-23t23-9h832q14 0 23 9t9 23zm128 448v-832q0-66-47-113t-113-47h-832q-66 0-113 47t-47 113v832q0 66 47 113t113 47h832q66 0 113-47t47-113zm128-832v832q0 119-84.5 203.5t-203.5 84.5h-832q-119 0-203.5-84.5t-84.5-203.5v-832q0-119 84.5-203.5t203.5-84.5h832q119 0 203.5 84.5t84.5 203.5z" })));
          } }]), S;
        })(f.a.PureComponent), De = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]), re = Le(_).style;
            return f.a.createElement("span", G, f.a.createElement("svg", { fill: re.color, width: re.height, height: re.width, style: re, viewBox: "0 0 1792 1792" }, f.a.createElement("path", { d: "M1344 800v64q0 14-9 23t-23 9h-352v352q0 14-9 23t-23 9h-64q-14 0-23-9t-9-23v-352h-352q-14 0-23-9t-9-23v-64q0-14 9-23t23-9h352v-352q0-14 9-23t23-9h64q14 0 23 9t9 23v352h352q14 0 23 9t9 23zm128 448v-832q0-66-47-113t-113-47h-832q-66 0-113 47t-47 113v832q0 66 47 113t113 47h832q66 0 113-47t47-113zm128-832v832q0 119-84.5 203.5t-203.5 84.5h-832q-119 0-203.5-84.5t-84.5-203.5v-832q0-119 84.5-203.5t203.5-84.5h832q119 0 203.5 84.5t84.5 203.5z" })));
          } }]), S;
        })(f.a.PureComponent), ze = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]);
            return f.a.createElement("span", G, f.a.createElement("svg", { style: l(l({}, Le(_).style), {}, { paddingLeft: "2px", verticalAlign: "top" }), viewBox: "0 0 15 15", fill: "currentColor" }, f.a.createElement("path", { d: "M0 14l6-6-6-6z" })));
          } }]), S;
        })(f.a.PureComponent), Pe = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]);
            return f.a.createElement("span", G, f.a.createElement("svg", { style: l(l({}, Le(_).style), {}, { paddingLeft: "2px", verticalAlign: "top" }), viewBox: "0 0 15 15", fill: "currentColor" }, f.a.createElement("path", { d: "M0 5l6 6 6-6z" })));
          } }]), S;
        })(f.a.PureComponent), ie = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]);
            return f.a.createElement("span", G, f.a.createElement("svg", Object.assign({}, Le(_), { viewBox: "0 0 40 40", fill: "currentColor", preserveAspectRatio: "xMidYMid meet" }), f.a.createElement("g", null, f.a.createElement("path", { d: "m30 35h-25v-22.5h25v7.5h2.5v-12.5c0-1.4-1.1-2.5-2.5-2.5h-7.5c0-2.8-2.2-5-5-5s-5 2.2-5 5h-7.5c-1.4 0-2.5 1.1-2.5 2.5v27.5c0 1.4 1.1 2.5 2.5 2.5h25c1.4 0 2.5-1.1 2.5-2.5v-5h-2.5v5z m-20-27.5h2.5s2.5-1.1 2.5-2.5 1.1-2.5 2.5-2.5 2.5 1.1 2.5 2.5 1.3 2.5 2.5 2.5h2.5s2.5 1.1 2.5 2.5h-20c0-1.5 1.1-2.5 2.5-2.5z m-2.5 20h5v-2.5h-5v2.5z m17.5-5v-5l-10 7.5 10 7.5v-5h12.5v-5h-12.5z m-17.5 10h7.5v-2.5h-7.5v2.5z m12.5-17.5h-12.5v2.5h12.5v-2.5z m-7.5 5h-5v2.5h5v-2.5z" }))));
          } }]), S;
        })(f.a.PureComponent), $e = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]);
            return f.a.createElement("span", G, f.a.createElement("svg", Object.assign({}, Le(_), { viewBox: "0 0 40 40", fill: "currentColor", preserveAspectRatio: "xMidYMid meet" }), f.a.createElement("g", null, f.a.createElement("path", { d: "m28.6 25q0-0.5-0.4-1l-4-4 4-4q0.4-0.5 0.4-1 0-0.6-0.4-1.1l-2-2q-0.4-0.4-1-0.4-0.6 0-1 0.4l-4.1 4.1-4-4.1q-0.4-0.4-1-0.4-0.6 0-1 0.4l-2 2q-0.5 0.5-0.5 1.1 0 0.5 0.5 1l4 4-4 4q-0.5 0.5-0.5 1 0 0.7 0.5 1.1l2 2q0.4 0.4 1 0.4 0.6 0 1-0.4l4-4.1 4.1 4.1q0.4 0.4 1 0.4 0.6 0 1-0.4l2-2q0.4-0.4 0.4-1z m8.7-5q0 4.7-2.3 8.6t-6.3 6.2-8.6 2.3-8.6-2.3-6.2-6.2-2.3-8.6 2.3-8.6 6.2-6.2 8.6-2.3 8.6 2.3 6.3 6.2 2.3 8.6z" }))));
          } }]), S;
        })(f.a.PureComponent), qe = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]);
            return f.a.createElement("span", G, f.a.createElement("svg", Object.assign({}, Le(_), { viewBox: "0 0 40 40", fill: "currentColor", preserveAspectRatio: "xMidYMid meet" }), f.a.createElement("g", null, f.a.createElement("path", { d: "m30.1 21.4v-2.8q0-0.6-0.4-1t-1-0.5h-5.7v-5.7q0-0.6-0.4-1t-1-0.4h-2.9q-0.6 0-1 0.4t-0.4 1v5.7h-5.7q-0.6 0-1 0.5t-0.5 1v2.8q0 0.6 0.5 1t1 0.5h5.7v5.7q0 0.5 0.4 1t1 0.4h2.9q0.6 0 1-0.4t0.4-1v-5.7h5.7q0.6 0 1-0.5t0.4-1z m7.2-1.4q0 4.7-2.3 8.6t-6.3 6.2-8.6 2.3-8.6-2.3-6.2-6.2-2.3-8.6 2.3-8.6 6.2-6.2 8.6-2.3 8.6 2.3 6.3 6.2 2.3 8.6z" }))));
          } }]), S;
        })(f.a.PureComponent), Je = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]);
            return f.a.createElement("span", G, f.a.createElement("svg", Object.assign({}, Le(_), { viewBox: "0 0 40 40", fill: "currentColor", preserveAspectRatio: "xMidYMid meet" }), f.a.createElement("g", null, f.a.createElement("path", { d: "m31.6 21.6h-10v10h-3.2v-10h-10v-3.2h10v-10h3.2v10h10v3.2z" }))));
          } }]), S;
        })(f.a.PureComponent), at = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]);
            return f.a.createElement("span", G, f.a.createElement("svg", Object.assign({}, Le(_), { viewBox: "0 0 40 40", fill: "currentColor", preserveAspectRatio: "xMidYMid meet" }), f.a.createElement("g", null, f.a.createElement("path", { d: "m19.8 26.4l2.6-2.6-3.4-3.4-2.6 2.6v1.3h2.2v2.1h1.2z m9.8-16q-0.3-0.4-0.7 0l-7.8 7.8q-0.4 0.4 0 0.7t0.7 0l7.8-7.8q0.4-0.4 0-0.7z m1.8 13.2v4.3q0 2.6-1.9 4.5t-4.5 1.9h-18.6q-2.6 0-4.5-1.9t-1.9-4.5v-18.6q0-2.7 1.9-4.6t4.5-1.8h18.6q1.4 0 2.6 0.5 0.3 0.2 0.4 0.5 0.1 0.4-0.2 0.7l-1.1 1.1q-0.3 0.3-0.7 0.1-0.5-0.1-1-0.1h-18.6q-1.4 0-2.5 1.1t-1 2.5v18.6q0 1.4 1 2.5t2.5 1h18.6q1.5 0 2.5-1t1.1-2.5v-2.9q0-0.2 0.2-0.4l1.4-1.5q0.3-0.3 0.8-0.1t0.4 0.6z m-2.1-16.5l6.4 6.5-15 15h-6.4v-6.5z m9.9 3l-2.1 2-6.4-6.4 2.1-2q0.6-0.7 1.5-0.7t1.5 0.7l3.4 3.4q0.6 0.6 0.6 1.5t-0.6 1.5z" }))));
          } }]), S;
        })(f.a.PureComponent), Ne = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.style, G = O(R, ["style"]);
            return f.a.createElement("span", G, f.a.createElement("svg", Object.assign({}, Le(_), { viewBox: "0 0 40 40", fill: "currentColor", preserveAspectRatio: "xMidYMid meet" }), f.a.createElement("g", null, f.a.createElement("path", { d: "m31.7 16.4q0-0.6-0.4-1l-2.1-2.1q-0.4-0.4-1-0.4t-1 0.4l-9.1 9.1-5-5q-0.5-0.4-1-0.4t-1 0.4l-2.1 2q-0.4 0.4-0.4 1 0 0.6 0.4 1l8.1 8.1q0.4 0.4 1 0.4 0.6 0 1-0.4l12.2-12.1q0.4-0.4 0.4-1z m5.6 3.6q0 4.7-2.3 8.6t-6.3 6.2-8.6 2.3-8.6-2.3-6.2-6.2-2.3-8.6 2.3-8.6 6.2-6.2 8.6-2.3 8.6 2.3 6.3 6.2 2.3 8.6z" }))));
          } }]), S;
        })(f.a.PureComponent);
        function Le(I) {
          return I || (I = {}), { style: l(l({ verticalAlign: "middle" }, I), {}, { color: I.color ? I.color : "#000000", height: "1em", width: "1em" }) };
        }
        var mt = (function(I) {
          h(S, I);
          var N = k(S);
          function S(R) {
            var _;
            return u(this, S), (_ = N.call(this, R)).copiedTimer = null, _.handleCopy = function() {
              var G = document.createElement("textarea"), re = _.props, ee = re.clickCallback, K = re.src, se = re.namespace;
              G.innerHTML = JSON.stringify(_.clipboardValue(K), null, "  "), document.body.appendChild(G), G.select(), document.execCommand("copy"), document.body.removeChild(G), _.copiedTimer = setTimeout((function() {
                _.setState({ copied: !1 });
              }), 5500), _.setState({ copied: !0 }, (function() {
                typeof ee == "function" && ee({ src: K, namespace: se, name: se[se.length - 1] });
              }));
            }, _.getClippyIcon = function() {
              var G = _.props.theme;
              return _.state.copied ? f.a.createElement("span", null, f.a.createElement(ie, Object.assign({ className: "copy-icon" }, $(G, "copy-icon"))), f.a.createElement("span", $(G, "copy-icon-copied"), "✔")) : f.a.createElement(ie, Object.assign({ className: "copy-icon" }, $(G, "copy-icon")));
            }, _.clipboardValue = function(G) {
              switch (P(G)) {
                case "function":
                case "regexp":
                  return G.toString();
                default:
                  return G;
              }
            }, _.state = { copied: !1 }, _;
          }
          return d(S, [{ key: "componentWillUnmount", value: function() {
            this.copiedTimer && (clearTimeout(this.copiedTimer), this.copiedTimer = null);
          } }, { key: "render", value: function() {
            var R = this.props, _ = (R.src, R.theme), G = R.hidden, re = R.rowHovered, ee = $(_, "copy-to-clipboard").style, K = "inline";
            return G && (K = "none"), f.a.createElement("span", { className: "copy-to-clipboard-container", title: "Copy to clipboard", style: { verticalAlign: "top", display: re ? "inline-block" : "none" } }, f.a.createElement("span", { style: l(l({}, ee), {}, { display: K }), onClick: this.handleCopy }, this.getClippyIcon()));
          } }]), S;
        })(f.a.PureComponent), ot = (function(I) {
          h(S, I);
          var N = k(S);
          function S(R) {
            var _;
            return u(this, S), (_ = N.call(this, R)).getEditIcon = function() {
              var G = _.props, re = G.variable, ee = G.theme;
              return f.a.createElement("div", { className: "click-to-edit", style: { verticalAlign: "top", display: _.state.hovered ? "inline-block" : "none" } }, f.a.createElement(at, Object.assign({ className: "click-to-edit-icon" }, $(ee, "editVarIcon"), { onClick: function() {
                _.prepopInput(re);
              } })));
            }, _.prepopInput = function(G) {
              if (_.props.onEdit !== !1) {
                var re = (function(K) {
                  var se;
                  switch (P(K)) {
                    case "undefined":
                      se = "undefined";
                      break;
                    case "nan":
                      se = "NaN";
                      break;
                    case "string":
                      se = K;
                      break;
                    case "date":
                    case "function":
                    case "regexp":
                      se = K.toString();
                      break;
                    default:
                      try {
                        se = JSON.stringify(K, null, "  ");
                      } catch {
                        se = "";
                      }
                  }
                  return se;
                })(G.value), ee = oe(re);
                _.setState({ editMode: !0, editValue: re, parsedInput: { type: ee.type, value: ee.value } });
              }
            }, _.getRemoveIcon = function() {
              var G = _.props, re = G.variable, ee = G.namespace, K = G.theme, se = G.rjvId;
              return f.a.createElement("div", { className: "click-to-remove", style: { verticalAlign: "top", display: _.state.hovered ? "inline-block" : "none" } }, f.a.createElement($e, Object.assign({ className: "click-to-remove-icon" }, $(K, "removeVarIcon"), { onClick: function() {
                fe.dispatch({ name: "VARIABLE_REMOVED", rjvId: se, data: { name: re.name, namespace: ee, existing_value: re.value, variable_removed: !0 } });
              } })));
            }, _.getValue = function(G, re) {
              var ee = !re && G.type, K = y(_).props;
              switch (ee) {
                case !1:
                  return _.getEditInput();
                case "string":
                  return f.a.createElement(Te, Object.assign({ value: G.value }, K));
                case "integer":
                  return f.a.createElement(ke, Object.assign({ value: G.value }, K));
                case "float":
                  return f.a.createElement(z, Object.assign({ value: G.value }, K));
                case "boolean":
                  return f.a.createElement(F, Object.assign({ value: G.value }, K));
                case "function":
                  return f.a.createElement(we, Object.assign({ value: G.value }, K));
                case "null":
                  return f.a.createElement(ge, K);
                case "nan":
                  return f.a.createElement(Ee, K);
                case "undefined":
                  return f.a.createElement(He, K);
                case "date":
                  return f.a.createElement(q, Object.assign({ value: G.value }, K));
                case "regexp":
                  return f.a.createElement(je, Object.assign({ value: G.value }, K));
                default:
                  return f.a.createElement("div", { className: "object-value" }, JSON.stringify(G.value));
              }
            }, _.getEditInput = function() {
              var G = _.props.theme, re = _.state.editValue;
              return f.a.createElement("div", null, f.a.createElement(te, Object.assign({ type: "text", inputRef: function(ee) {
                return ee && ee.focus();
              }, value: re, className: "variable-editor", onChange: function(ee) {
                var K = ee.target.value, se = oe(K);
                _.setState({ editValue: K, parsedInput: { type: se.type, value: se.value } });
              }, onKeyDown: function(ee) {
                switch (ee.key) {
                  case "Escape":
                    _.setState({ editMode: !1, editValue: "" });
                    break;
                  case "Enter":
                    (ee.ctrlKey || ee.metaKey) && _.submitEdit(!0);
                }
                ee.stopPropagation();
              }, placeholder: "update this value", minRows: 2 }, $(G, "edit-input"))), f.a.createElement("div", $(G, "edit-icon-container"), f.a.createElement($e, Object.assign({ className: "edit-cancel" }, $(G, "cancel-icon"), { onClick: function() {
                _.setState({ editMode: !1, editValue: "" });
              } })), f.a.createElement(Ne, Object.assign({ className: "edit-check string-value" }, $(G, "check-icon"), { onClick: function() {
                _.submitEdit();
              } })), f.a.createElement("div", null, _.showDetected())));
            }, _.submitEdit = function(G) {
              var re = _.props, ee = re.variable, K = re.namespace, se = re.rjvId, Oe = _.state, Xe = Oe.editValue, nt = Oe.parsedInput, lt = Xe;
              G && nt.type && (lt = nt.value), _.setState({ editMode: !1 }), fe.dispatch({ name: "VARIABLE_UPDATED", rjvId: se, data: { name: ee.name, namespace: K, existing_value: ee.value, new_value: lt, variable_removed: !1 } });
            }, _.showDetected = function() {
              var G = _.props, re = G.theme, ee = (G.variable, G.namespace, G.rjvId, _.state.parsedInput), K = (ee.type, ee.value, _.getDetectedInput());
              if (K) return f.a.createElement("div", null, f.a.createElement("div", $(re, "detected-row"), K, f.a.createElement(Ne, { className: "edit-check detected", style: l({ verticalAlign: "top", paddingLeft: "3px" }, $(re, "check-icon").style), onClick: function() {
                _.submitEdit(!0);
              } })));
            }, _.getDetectedInput = function() {
              var G = _.state.parsedInput, re = G.type, ee = G.value, K = y(_).props, se = K.theme;
              if (re !== !1) switch (re.toLowerCase()) {
                case "object":
                  return f.a.createElement("span", null, f.a.createElement("span", { style: l(l({}, $(se, "brace").style), {}, { cursor: "default" }) }, "{"), f.a.createElement("span", { style: l(l({}, $(se, "ellipsis").style), {}, { cursor: "default" }) }, "..."), f.a.createElement("span", { style: l(l({}, $(se, "brace").style), {}, { cursor: "default" }) }, "}"));
                case "array":
                  return f.a.createElement("span", null, f.a.createElement("span", { style: l(l({}, $(se, "brace").style), {}, { cursor: "default" }) }, "["), f.a.createElement("span", { style: l(l({}, $(se, "ellipsis").style), {}, { cursor: "default" }) }, "..."), f.a.createElement("span", { style: l(l({}, $(se, "brace").style), {}, { cursor: "default" }) }, "]"));
                case "string":
                  return f.a.createElement(Te, Object.assign({ value: ee }, K));
                case "integer":
                  return f.a.createElement(ke, Object.assign({ value: ee }, K));
                case "float":
                  return f.a.createElement(z, Object.assign({ value: ee }, K));
                case "boolean":
                  return f.a.createElement(F, Object.assign({ value: ee }, K));
                case "function":
                  return f.a.createElement(we, Object.assign({ value: ee }, K));
                case "null":
                  return f.a.createElement(ge, K);
                case "nan":
                  return f.a.createElement(Ee, K);
                case "undefined":
                  return f.a.createElement(He, K);
                case "date":
                  return f.a.createElement(q, Object.assign({ value: new Date(ee) }, K));
              }
            }, _.state = { editMode: !1, editValue: "", hovered: !1, renameKey: !1, parsedInput: { type: !1, value: null } }, _;
          }
          return d(S, [{ key: "render", value: function() {
            var R = this, _ = this.props, G = _.variable, re = _.singleIndent, ee = _.type, K = _.theme, se = _.namespace, Oe = _.indentWidth, Xe = _.enableClipboard, nt = _.onEdit, lt = _.onDelete, Ge = _.onSelect, ct = _.displayArrayKey, yt = _.quotesOnKeys, Be = this.state.editMode;
            return f.a.createElement("div", Object.assign({}, $(K, "objectKeyVal", { paddingLeft: Oe * re }), { onMouseEnter: function() {
              return R.setState(l(l({}, R.state), {}, { hovered: !0 }));
            }, onMouseLeave: function() {
              return R.setState(l(l({}, R.state), {}, { hovered: !1 }));
            }, className: "variable-row", key: G.name }), ee == "array" ? ct ? f.a.createElement("span", Object.assign({}, $(K, "array-key"), { key: G.name + "_" + se }), G.name, f.a.createElement("div", $(K, "colon"), ":")) : null : f.a.createElement("span", null, f.a.createElement("span", Object.assign({}, $(K, "object-name"), { className: "object-key", key: G.name + "_" + se }), !!yt && f.a.createElement("span", { style: { verticalAlign: "top" } }, '"'), f.a.createElement("span", { style: { display: "inline-block" } }, G.name), !!yt && f.a.createElement("span", { style: { verticalAlign: "top" } }, '"')), f.a.createElement("span", $(K, "colon"), ":")), f.a.createElement("div", Object.assign({ className: "variable-value", onClick: Ge === !1 && nt === !1 ? null : function($t) {
              var _t = de(se);
              ($t.ctrlKey || $t.metaKey) && nt !== !1 ? R.prepopInput(G) : Ge !== !1 && (_t.shift(), Ge(l(l({}, G), {}, { namespace: _t })));
            } }, $(K, "variableValue", { cursor: Ge === !1 ? "default" : "pointer" })), this.getValue(G, Be)), Xe ? f.a.createElement(mt, { rowHovered: this.state.hovered, hidden: Be, src: G.value, clickCallback: Xe, theme: K, namespace: [].concat(de(se), [G.name]) }) : null, nt !== !1 && Be == 0 ? this.getEditIcon() : null, lt !== !1 && Be == 0 ? this.getRemoveIcon() : null);
          } }]), S;
        })(f.a.PureComponent), rt = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            var R;
            u(this, S);
            for (var _ = arguments.length, G = new Array(_), re = 0; re < _; re++) G[re] = arguments[re];
            return (R = N.call.apply(N, [this].concat(G))).getObjectSize = function() {
              var ee = R.props, K = ee.size, se = ee.theme;
              if (ee.displayObjectSize) return f.a.createElement("span", Object.assign({ className: "object-size" }, $(se, "object-size")), K, " item", K === 1 ? "" : "s");
            }, R.getAddAttribute = function(ee) {
              var K = R.props, se = K.theme, Oe = K.namespace, Xe = K.name, nt = K.src, lt = K.rjvId, Ge = K.depth;
              return f.a.createElement("span", { className: "click-to-add", style: { verticalAlign: "top", display: ee ? "inline-block" : "none" } }, f.a.createElement(qe, Object.assign({ className: "click-to-add-icon" }, $(se, "addVarIcon"), { onClick: function() {
                var ct = { name: Ge > 0 ? Xe : null, namespace: Oe.splice(0, Oe.length - 1), existing_value: nt, variable_removed: !1, key_name: null };
                P(nt) === "object" ? fe.dispatch({ name: "ADD_VARIABLE_KEY_REQUEST", rjvId: lt, data: ct }) : fe.dispatch({ name: "VARIABLE_ADDED", rjvId: lt, data: l(l({}, ct), {}, { new_value: [].concat(de(nt), [null]) }) });
              } })));
            }, R.getRemoveObject = function(ee) {
              var K = R.props, se = K.theme, Oe = (K.hover, K.namespace), Xe = K.name, nt = K.src, lt = K.rjvId;
              if (Oe.length !== 1) return f.a.createElement("span", { className: "click-to-remove", style: { display: ee ? "inline-block" : "none" } }, f.a.createElement($e, Object.assign({ className: "click-to-remove-icon" }, $(se, "removeVarIcon"), { onClick: function() {
                fe.dispatch({ name: "VARIABLE_REMOVED", rjvId: lt, data: { name: Xe, namespace: Oe.splice(0, Oe.length - 1), existing_value: nt, variable_removed: !0 } });
              } })));
            }, R.render = function() {
              var ee = R.props, K = ee.theme, se = ee.onDelete, Oe = ee.onAdd, Xe = ee.enableClipboard, nt = ee.src, lt = ee.namespace, Ge = ee.rowHovered;
              return f.a.createElement("div", Object.assign({}, $(K, "object-meta-data"), { className: "object-meta-data", onClick: function(ct) {
                ct.stopPropagation();
              } }), R.getObjectSize(), Xe ? f.a.createElement(mt, { rowHovered: Ge, clickCallback: Xe, src: nt, theme: K, namespace: lt }) : null, Oe !== !1 ? R.getAddAttribute(Ge) : null, se !== !1 ? R.getRemoveObject(Ge) : null);
            }, R;
          }
          return S;
        })(f.a.PureComponent);
        function Ze(I) {
          var N = I.parent_type, S = I.namespace, R = I.quotesOnKeys, _ = I.theme, G = I.jsvRoot, re = I.name, ee = I.displayArrayKey, K = I.name ? I.name : "";
          return !G || re !== !1 && re !== null ? N == "array" ? ee ? f.a.createElement("span", Object.assign({}, $(_, "array-key"), { key: S }), f.a.createElement("span", { className: "array-key" }, K), f.a.createElement("span", $(_, "colon"), ":")) : f.a.createElement("span", null) : f.a.createElement("span", Object.assign({}, $(_, "object-name"), { key: S }), f.a.createElement("span", { className: "object-key" }, R && f.a.createElement("span", { style: { verticalAlign: "top" } }, '"'), f.a.createElement("span", null, K), R && f.a.createElement("span", { style: { verticalAlign: "top" } }, '"')), f.a.createElement("span", $(_, "colon"), ":")) : f.a.createElement("span", null);
        }
        function bt(I) {
          var N = I.theme;
          switch (I.iconStyle) {
            case "triangle":
              return f.a.createElement(Pe, Object.assign({}, $(N, "expanded-icon"), { className: "expanded-icon" }));
            case "square":
              return f.a.createElement(me, Object.assign({}, $(N, "expanded-icon"), { className: "expanded-icon" }));
            default:
              return f.a.createElement(Me, Object.assign({}, $(N, "expanded-icon"), { className: "expanded-icon" }));
          }
        }
        function Nt(I) {
          var N = I.theme;
          switch (I.iconStyle) {
            case "triangle":
              return f.a.createElement(ze, Object.assign({}, $(N, "collapsed-icon"), { className: "collapsed-icon" }));
            case "square":
              return f.a.createElement(De, Object.assign({}, $(N, "collapsed-icon"), { className: "collapsed-icon" }));
            default:
              return f.a.createElement(Ce, Object.assign({}, $(N, "collapsed-icon"), { className: "collapsed-icon" }));
          }
        }
        var Lt = (function(I) {
          h(S, I);
          var N = k(S);
          function S(R) {
            var _;
            return u(this, S), (_ = N.call(this, R)).toggleCollapsed = function(G) {
              var re = [];
              for (var ee in _.state.expanded) re.push(_.state.expanded[ee]);
              re[G] = !re[G], _.setState({ expanded: re });
            }, _.state = { expanded: [] }, _;
          }
          return d(S, [{ key: "getExpandedIcon", value: function(R) {
            var _ = this.props, G = _.theme, re = _.iconStyle;
            return this.state.expanded[R] ? f.a.createElement(bt, { theme: G, iconStyle: re }) : f.a.createElement(Nt, { theme: G, iconStyle: re });
          } }, { key: "render", value: function() {
            var R = this, _ = this.props, G = _.src, re = _.groupArraysAfterLength, ee = (_.depth, _.name), K = _.theme, se = _.jsvRoot, Oe = _.namespace, Xe = (_.parent_type, O(_, ["src", "groupArraysAfterLength", "depth", "name", "theme", "jsvRoot", "namespace", "parent_type"])), nt = 0, lt = 5 * this.props.indentWidth;
            se || (nt = 5 * this.props.indentWidth);
            var Ge = re, ct = Math.ceil(G.length / Ge);
            return f.a.createElement("div", Object.assign({ className: "object-key-val" }, $(K, se ? "jsv-root" : "objectKeyVal", { paddingLeft: nt })), f.a.createElement(Ze, this.props), f.a.createElement("span", null, f.a.createElement(rt, Object.assign({ size: G.length }, this.props))), de(Array(ct)).map((function(yt, Be) {
              return f.a.createElement("div", Object.assign({ key: Be, className: "object-key-val array-group" }, $(K, "objectKeyVal", { marginLeft: 6, paddingLeft: lt })), f.a.createElement("span", $(K, "brace-row"), f.a.createElement("div", Object.assign({ className: "icon-container" }, $(K, "icon-container"), { onClick: function($t) {
                R.toggleCollapsed(Be);
              } }), R.getExpandedIcon(Be)), R.state.expanded[Be] ? f.a.createElement(qr, Object.assign({ key: ee + Be, depth: 0, name: !1, collapsed: !1, groupArraysAfterLength: Ge, index_offset: Be * Ge, src: G.slice(Be * Ge, Be * Ge + Ge), namespace: Oe, type: "array", parent_type: "array_group", theme: K }, Xe)) : f.a.createElement("span", Object.assign({}, $(K, "brace"), { onClick: function($t) {
                R.toggleCollapsed(Be);
              }, className: "array-group-brace" }), "[", f.a.createElement("div", Object.assign({}, $(K, "array-group-meta-data"), { className: "array-group-meta-data" }), f.a.createElement("span", Object.assign({ className: "object-size" }, $(K, "object-size")), Be * Ge, " - ", Be * Ge + Ge > G.length ? G.length : Be * Ge + Ge)), "]")));
            })));
          } }]), S;
        })(f.a.PureComponent), _r = (function(I) {
          h(S, I);
          var N = k(S);
          function S(R) {
            var _;
            u(this, S), (_ = N.call(this, R)).toggleCollapsed = function() {
              _.setState({ expanded: !_.state.expanded }, (function() {
                be.set(_.props.rjvId, _.props.namespace, "expanded", _.state.expanded);
              }));
            }, _.getObjectContent = function(re, ee, K) {
              return f.a.createElement("div", { className: "pushed-content object-container" }, f.a.createElement("div", Object.assign({ className: "object-content" }, $(_.props.theme, "pushed-content")), _.renderObjectContents(ee, K)));
            }, _.getEllipsis = function() {
              return _.state.size === 0 ? null : f.a.createElement("div", Object.assign({}, $(_.props.theme, "ellipsis"), { className: "node-ellipsis", onClick: _.toggleCollapsed }), "...");
            }, _.getObjectMetaData = function(re) {
              var ee = _.props, K = (ee.rjvId, ee.theme, _.state), se = K.size, Oe = K.hovered;
              return f.a.createElement(rt, Object.assign({ rowHovered: Oe, size: se }, _.props));
            }, _.renderObjectContents = function(re, ee) {
              var K, se = _.props, Oe = se.depth, Xe = se.parent_type, nt = se.index_offset, lt = se.groupArraysAfterLength, Ge = se.namespace, ct = _.state.object_type, yt = [], Be = Object.keys(re || {});
              return _.props.sortKeys && ct !== "array" && (Be = Be.sort()), Be.forEach((function($t) {
                if (K = new xo($t, re[$t]), Xe === "array_group" && nt && (K.name = parseInt(K.name) + nt), re.hasOwnProperty($t)) if (K.type === "object") yt.push(f.a.createElement(qr, Object.assign({ key: K.name, depth: Oe + 1, name: K.name, src: K.value, namespace: Ge.concat(K.name), parent_type: ct }, ee)));
                else if (K.type === "array") {
                  var _t = qr;
                  lt && K.value.length > lt && (_t = Lt), yt.push(f.a.createElement(_t, Object.assign({ key: K.name, depth: Oe + 1, name: K.name, src: K.value, namespace: Ge.concat(K.name), type: "array", parent_type: ct }, ee)));
                } else yt.push(f.a.createElement(ot, Object.assign({ key: K.name + "_" + Ge, variable: K, singleIndent: 5, namespace: Ge, type: _.props.type }, ee)));
              })), yt;
            };
            var G = S.getState(R);
            return _.state = l(l({}, G), {}, { prevProps: {} }), _;
          }
          return d(S, [{ key: "getBraceStart", value: function(R, _) {
            var G = this, re = this.props, ee = re.src, K = re.theme, se = re.iconStyle;
            if (re.parent_type === "array_group") return f.a.createElement("span", null, f.a.createElement("span", $(K, "brace"), R === "array" ? "[" : "{"), _ ? this.getObjectMetaData(ee) : null);
            var Oe = _ ? bt : Nt;
            return f.a.createElement("span", null, f.a.createElement("span", Object.assign({ onClick: function(Xe) {
              G.toggleCollapsed();
            } }, $(K, "brace-row")), f.a.createElement("div", Object.assign({ className: "icon-container" }, $(K, "icon-container")), f.a.createElement(Oe, { theme: K, iconStyle: se })), f.a.createElement(Ze, this.props), f.a.createElement("span", $(K, "brace"), R === "array" ? "[" : "{")), _ ? this.getObjectMetaData(ee) : null);
          } }, { key: "render", value: function() {
            var R = this, _ = this.props, G = _.depth, re = _.src, ee = (_.namespace, _.name, _.type, _.parent_type), K = _.theme, se = _.jsvRoot, Oe = _.iconStyle, Xe = O(_, ["depth", "src", "namespace", "name", "type", "parent_type", "theme", "jsvRoot", "iconStyle"]), nt = this.state, lt = nt.object_type, Ge = nt.expanded, ct = {};
            return se || ee === "array_group" ? ee === "array_group" && (ct.borderLeft = 0, ct.display = "inline") : ct.paddingLeft = 5 * this.props.indentWidth, f.a.createElement("div", Object.assign({ className: "object-key-val", onMouseEnter: function() {
              return R.setState(l(l({}, R.state), {}, { hovered: !0 }));
            }, onMouseLeave: function() {
              return R.setState(l(l({}, R.state), {}, { hovered: !1 }));
            } }, $(K, se ? "jsv-root" : "objectKeyVal", ct)), this.getBraceStart(lt, Ge), Ge ? this.getObjectContent(G, re, l({ theme: K, iconStyle: Oe }, Xe)) : this.getEllipsis(), f.a.createElement("span", { className: "brace-row" }, f.a.createElement("span", { style: l(l({}, $(K, "brace").style), {}, { paddingLeft: Ge ? "3px" : "0px" }) }, lt === "array" ? "]" : "}"), Ge ? null : this.getObjectMetaData(re)));
          } }], [{ key: "getDerivedStateFromProps", value: function(R, _) {
            var G = _.prevProps;
            return R.src !== G.src || R.collapsed !== G.collapsed || R.name !== G.name || R.namespace !== G.namespace || R.rjvId !== G.rjvId ? l(l({}, S.getState(R)), {}, { prevProps: R }) : null;
          } }]), S;
        })(f.a.PureComponent);
        _r.getState = function(I) {
          var N = Object.keys(I.src).length, S = (I.collapsed === !1 || I.collapsed !== !0 && I.collapsed > I.depth) && (!I.shouldCollapse || I.shouldCollapse({ name: I.name, src: I.src, type: P(I.src), namespace: I.namespace }) === !1) && N !== 0;
          return { expanded: be.get(I.rjvId, I.namespace, "expanded", S), object_type: I.type === "array" ? "array" : "object", parent_type: I.type === "array" ? "array" : "object", size: N, hovered: !1 };
        };
        var xo = function I(N, S) {
          u(this, I), this.name = N, this.value = S, this.type = P(S);
        };
        W(_r);
        var qr = _r, Co = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            var R;
            u(this, S);
            for (var _ = arguments.length, G = new Array(_), re = 0; re < _; re++) G[re] = arguments[re];
            return (R = N.call.apply(N, [this].concat(G))).render = function() {
              var ee = y(R).props, K = [ee.name], se = qr;
              return Array.isArray(ee.src) && ee.groupArraysAfterLength && ee.src.length > ee.groupArraysAfterLength && (se = Lt), f.a.createElement("div", { className: "pretty-json-container object-container" }, f.a.createElement("div", { className: "object-content" }, f.a.createElement(se, Object.assign({ namespace: K, depth: 0, jsvRoot: !0 }, ee))));
            }, R;
          }
          return S;
        })(f.a.PureComponent), So = (function(I) {
          h(S, I);
          var N = k(S);
          function S(R) {
            var _;
            return u(this, S), (_ = N.call(this, R)).closeModal = function() {
              fe.dispatch({ rjvId: _.props.rjvId, name: "RESET" });
            }, _.submit = function() {
              _.props.submit(_.state.input);
            }, _.state = { input: R.input ? R.input : "" }, _;
          }
          return d(S, [{ key: "render", value: function() {
            var R = this, _ = this.props, G = _.theme, re = _.rjvId, ee = _.isValid, K = this.state.input, se = ee(K);
            return f.a.createElement("div", Object.assign({ className: "key-modal-request" }, $(G, "key-modal-request"), { onClick: this.closeModal }), f.a.createElement("div", Object.assign({}, $(G, "key-modal"), { onClick: function(Oe) {
              Oe.stopPropagation();
            } }), f.a.createElement("div", $(G, "key-modal-label"), "Key Name:"), f.a.createElement("div", { style: { position: "relative" } }, f.a.createElement("input", Object.assign({}, $(G, "key-modal-input"), { className: "key-modal-input", ref: function(Oe) {
              return Oe && Oe.focus();
            }, spellCheck: !1, value: K, placeholder: "...", onChange: function(Oe) {
              R.setState({ input: Oe.target.value });
            }, onKeyPress: function(Oe) {
              se && Oe.key === "Enter" ? R.submit() : Oe.key === "Escape" && R.closeModal();
            } })), se ? f.a.createElement(Ne, Object.assign({}, $(G, "key-modal-submit"), { className: "key-modal-submit", onClick: function(Oe) {
              return R.submit();
            } })) : null), f.a.createElement("span", $(G, "key-modal-cancel"), f.a.createElement(Je, Object.assign({}, $(G, "key-modal-cancel-icon"), { className: "key-modal-cancel", onClick: function() {
              fe.dispatch({ rjvId: re, name: "RESET" });
            } })))));
          } }]), S;
        })(f.a.PureComponent), Gt = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            var R;
            u(this, S);
            for (var _ = arguments.length, G = new Array(_), re = 0; re < _; re++) G[re] = arguments[re];
            return (R = N.call.apply(N, [this].concat(G))).isValid = function(ee) {
              var K = R.props.rjvId, se = be.get(K, "action", "new-key-request");
              return ee != "" && Object.keys(se.existing_value).indexOf(ee) === -1;
            }, R.submit = function(ee) {
              var K = R.props.rjvId, se = be.get(K, "action", "new-key-request");
              se.new_value = l({}, se.existing_value), se.new_value[ee] = R.props.defaultValue, fe.dispatch({ name: "VARIABLE_ADDED", rjvId: K, data: se });
            }, R;
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.active, G = R.theme, re = R.rjvId;
            return _ ? f.a.createElement(So, { rjvId: re, theme: G, isValid: this.isValid, submit: this.submit }) : null;
          } }]), S;
        })(f.a.PureComponent), wo = (function(I) {
          h(S, I);
          var N = k(S);
          function S() {
            return u(this, S), N.apply(this, arguments);
          }
          return d(S, [{ key: "render", value: function() {
            var R = this.props, _ = R.message, G = R.active, re = R.theme, ee = R.rjvId;
            return G ? f.a.createElement("div", Object.assign({ className: "validation-failure" }, $(re, "validation-failure"), { onClick: function() {
              fe.dispatch({ rjvId: ee, name: "RESET" });
            } }), f.a.createElement("span", $(re, "validation-failure-label"), _), f.a.createElement(Je, $(re, "validation-failure-clear"))) : null;
          } }]), S;
        })(f.a.PureComponent), Gr = (function(I) {
          h(S, I);
          var N = k(S);
          function S(R) {
            var _;
            return u(this, S), (_ = N.call(this, R)).rjvId = Date.now().toString(), _.getListeners = function() {
              return { reset: _.resetState, "variable-update": _.updateSrc, "add-key-request": _.addKeyRequest };
            }, _.updateSrc = function() {
              var G, re = be.get(_.rjvId, "action", "variable-update"), ee = re.name, K = re.namespace, se = re.new_value, Oe = re.existing_value, Xe = (re.variable_removed, re.updated_src), nt = re.type, lt = _.props, Ge = lt.onEdit, ct = lt.onDelete, yt = lt.onAdd, Be = { existing_src: _.state.src, new_value: se, updated_src: Xe, name: ee, namespace: K, existing_value: Oe };
              switch (nt) {
                case "variable-added":
                  G = yt(Be);
                  break;
                case "variable-edited":
                  G = Ge(Be);
                  break;
                case "variable-removed":
                  G = ct(Be);
              }
              G !== !1 ? (be.set(_.rjvId, "global", "src", Xe), _.setState({ src: Xe })) : _.setState({ validationFailure: !0 });
            }, _.addKeyRequest = function() {
              _.setState({ addKeyRequest: !0 });
            }, _.resetState = function() {
              _.setState({ validationFailure: !1, addKeyRequest: !1 });
            }, _.state = { addKeyRequest: !1, editKeyRequest: !1, validationFailure: !1, src: S.defaultProps.src, name: S.defaultProps.name, theme: S.defaultProps.theme, validationMessage: S.defaultProps.validationMessage, prevSrc: S.defaultProps.src, prevName: S.defaultProps.name, prevTheme: S.defaultProps.theme }, _;
          }
          return d(S, [{ key: "componentDidMount", value: function() {
            be.set(this.rjvId, "global", "src", this.state.src);
            var R = this.getListeners();
            for (var _ in R) be.on(_ + "-" + this.rjvId, R[_]);
            this.setState({ addKeyRequest: !1, editKeyRequest: !1 });
          } }, { key: "componentDidUpdate", value: function(R, _) {
            _.addKeyRequest !== !1 && this.setState({ addKeyRequest: !1 }), _.editKeyRequest !== !1 && this.setState({ editKeyRequest: !1 }), R.src !== this.state.src && be.set(this.rjvId, "global", "src", this.state.src);
          } }, { key: "componentWillUnmount", value: function() {
            var R = this.getListeners();
            for (var _ in R) be.removeListener(_ + "-" + this.rjvId, R[_]);
          } }, { key: "render", value: function() {
            var R = this.state, _ = R.validationFailure, G = R.validationMessage, re = R.addKeyRequest, ee = R.theme, K = R.src, se = R.name, Oe = this.props, Xe = Oe.style, nt = Oe.defaultValue;
            return f.a.createElement("div", { className: "react-json-view", style: l(l({}, $(ee, "app-container").style), Xe) }, f.a.createElement(wo, { message: G, active: _, theme: ee, rjvId: this.rjvId }), f.a.createElement(Co, Object.assign({}, this.props, { src: K, name: se, theme: ee, type: P(K), rjvId: this.rjvId })), f.a.createElement(Gt, { active: re, theme: ee, rjvId: this.rjvId, defaultValue: nt }));
          } }], [{ key: "getDerivedStateFromProps", value: function(R, _) {
            if (R.src !== _.prevSrc || R.name !== _.prevName || R.theme !== _.prevTheme) {
              var G = { src: R.src, name: R.name, theme: R.theme, validationMessage: R.validationMessage, prevSrc: R.src, prevName: R.name, prevTheme: R.theme };
              return S.validateState(G);
            }
            return null;
          } }]), S;
        })(f.a.PureComponent);
        Gr.defaultProps = { src: {}, name: "root", theme: "rjv-default", collapsed: !1, collapseStringsAfterLength: !1, shouldCollapse: !1, sortKeys: !1, quotesOnKeys: !0, groupArraysAfterLength: 100, indentWidth: 4, enableClipboard: !0, displayObjectSize: !0, displayDataTypes: !0, onEdit: !1, onDelete: !1, onAdd: !1, onSelect: !1, iconStyle: "triangle", style: {}, validationMessage: "Validation Error", defaultValue: null, displayArrayKey: !0 }, Gr.validateState = function(I) {
          var N = {};
          return P(I.theme) !== "object" || (function(S) {
            var R = ["base00", "base01", "base02", "base03", "base04", "base05", "base06", "base07", "base08", "base09", "base0A", "base0B", "base0C", "base0D", "base0E", "base0F"];
            if (P(S) === "object") {
              for (var _ = 0; _ < R.length; _++) if (!(R[_] in S)) return !1;
              return !0;
            }
            return !1;
          })(I.theme) || (console.error("react-json-view error:", "theme prop must be a theme name or valid base-16 theme object.", 'defaulting to "rjv-default" theme'), N.theme = "rjv-default"), P(I.src) !== "object" && P(I.src) !== "array" && (console.error("react-json-view error:", "src property must be a valid json object"), N.name = "ERROR", N.src = { message: "src property must be a valid json object" }), l(l({}, I), N);
        }, W(Gr), n.default = Gr;
      }]);
    }));
  })(Aa)), Aa.exports;
}
var Fb = Bb();
const zb = /* @__PURE__ */ Tr(Fb);
function Wb(t) {
  const { maxHeight: e = "initial" } = t;
  return /* @__PURE__ */ B.jsx(ve, { maxHeight: e, overflow: "auto", width: "100%", children: /* @__PURE__ */ B.jsx(zb, { ...t }) });
}
const Mi = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z"
})), Du = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M10.08 10.86c.05-.33.16-.62.3-.87s.34-.46.59-.62c.24-.15.54-.22.91-.23.23.01.44.05.63.13.2.09.38.21.52.36s.25.33.34.53.13.42.14.64h1.79c-.02-.47-.11-.9-.28-1.29s-.4-.73-.7-1.01-.66-.5-1.08-.66-.88-.23-1.39-.23c-.65 0-1.22.11-1.7.34s-.88.53-1.2.92-.56.84-.71 1.36S8 11.29 8 11.87v.27c0 .58.08 1.12.23 1.64s.39.97.71 1.35.72.69 1.2.91c.48.22 1.05.34 1.7.34.47 0 .91-.08 1.32-.23s.77-.36 1.08-.63.56-.58.74-.94.29-.74.3-1.15h-1.79c-.01.21-.06.4-.15.58s-.21.33-.36.46-.32.23-.52.3c-.19.07-.39.09-.6.1-.36-.01-.66-.08-.89-.23-.25-.16-.45-.37-.59-.62s-.25-.55-.3-.88-.08-.67-.08-1v-.27c0-.35.03-.68.08-1.01zM12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z"
})), Vb = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"
})), Hb = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"
})), Kb = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M8 5v14l11-7z"
})), Ub = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M21 6h-2v9H6v2c0 .55.45 1 1 1h11l4 4V7c0-.55-.45-1-1-1zm-4 6V3c0-.55-.45-1-1-1H3c-.55 0-1 .45-1 1v14l4-4h10c.55 0 1-.45 1-1z"
})), qb = Jt(/* @__PURE__ */ b.createElement("path", {
  d: "M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"
})), Gb = "Message", Bu = (t) => (t == null ? void 0 : t.replace(/\s/g, " ")) ?? "";
function Yb(t) {
  const { title: e, value: a } = t, r = Lu(), o = (() => {
    switch (typeof a) {
      case "string":
        return Bu(a);
      case "number":
      case "boolean":
      case "bigint":
        return a.toString();
      case "object":
        return a ? /* @__PURE__ */ B.jsx(
          Wb,
          {
            displayDataTypes: !1,
            displayObjectSize: !1,
            shouldCollapse: !1,
            src: a,
            maxHeight: "100%",
            name: !1,
            enableClipboard: !1
          }
        ) : null;
      default:
        return "";
    }
  })();
  return !o || e === "randomKey" || e === "isNewLog" || e === "traceAvailable" || e === "displayTimeStamp" || e === "apiName" || !e ? null : /* @__PURE__ */ B.jsxs("div", { className: r.expandedLogRow, children: [
    /* @__PURE__ */ B.jsx("span", { className: r.expandedLogKey, children: e }),
    /* @__PURE__ */ B.jsx("span", { children: o })
  ] });
}
function Xb(t) {
  var m;
  const {
    logEntry: e,
    timeZone: a,
    onCopy: r,
    getColor: n,
    isHeiglighted: o,
    isExpanded: i,
    isExpandable: s
  } = t, l = Lu(), u = "Copy", c = "Copy Selection", d = (y) => {
    const w = parseInt(y, 10);
    return w >= 200 && w < 300 ? l.infoLogs : w >= 300 && w < 400 ? l.warnLogs : w >= 400 && w < 600 ? l.errorLogs : l.logOther;
  }, p = (y) => {
    const w = y.toUpperCase();
    return w === Ib ? l.infoLogs : w === Ab ? l.warningLogs : w === Mb || w === Nb ? l.errorLogs : w === Lb ? l.infoLogs : d(w);
  }, h = () => {
    r(e);
  }, g = (y) => {
    y.preventDefault(), y.stopPropagation();
  };
  return /* @__PURE__ */ B.jsxs("div", { children: [
    /* @__PURE__ */ B.jsxs(
      "div",
      {
        className: he(l.defaultLogLine, {
          [l.highlightedLogLine]: e.isNewLog && !o,
          [l.selectedLogLine]: o,
          [l.whiteBackground]: !(o || e.isNewLog)
        }),
        "data-testid": "log-panel-entry",
        children: [
          s ? /* @__PURE__ */ B.jsx("div", { className: l.arrow, children: i ? /* @__PURE__ */ B.jsx(Mi, { fontSize: "inherit", rotate: 90 }) : /* @__PURE__ */ B.jsx(Mi, { fontSize: "inherit" }) }) : /* @__PURE__ */ B.jsx("div", { className: l.arrow }),
          /* @__PURE__ */ B.jsxs("div", { className: l.logLineWrapper, children: [
            /* @__PURE__ */ B.jsxs("span", { className: l.infoSection, children: [
              /* @__PURE__ */ B.jsxs(
                "span",
                {
                  className: he(
                    p(jb),
                    l.toolTipParent
                  ),
                  children: [
                    Pb(new Date(e.timestamp), a),
                    " "
                  ]
                }
              ),
              (e == null ? void 0 : e.message) && /* @__PURE__ */ B.jsxs(
                "span",
                {
                  className: he(
                    (m = e == null ? void 0 : e.message) != null && m.includes("Error") ? l.errorLogs : n(e == null ? void 0 : e.message),
                    l.toolTipParent
                  ),
                  children: [
                    e == null ? void 0 : e.message,
                    " ",
                    /* @__PURE__ */ B.jsx("span", { className: l.toolTipChild, children: Gb })
                  ]
                }
              )
            ] }),
            /* @__PURE__ */ B.jsx("span", { className: l.logMsg, children: /* @__PURE__ */ B.jsx(
              "span",
              {
                className: he(
                  l.disableUserSelect,
                  l.logEntryActions
                ),
                children: /* @__PURE__ */ B.jsx("span", { className: l.actionContainer, children: /* @__PURE__ */ B.jsxs(
                  "button",
                  {
                    onClick: (y) => {
                      g(y), h();
                    },
                    onMouseDown: g,
                    onMouseUp: g,
                    onMouseEnter: g,
                    type: "button",
                    className: l.actionButton,
                    children: [
                      /* @__PURE__ */ B.jsx(Du, { className: l.actionIcon }),
                      /* @__PURE__ */ B.jsx("span", { className: l.actionLabel, children: o ? c : u })
                    ]
                  }
                ) })
              }
            ) })
          ] })
        ]
      },
      e.randomKey
    ),
    i && /* @__PURE__ */ B.jsx("div", { className: l.tableContainer, children: Object.entries(e).map(([y, w]) => /* @__PURE__ */ B.jsx(
      Yb,
      {
        title: y,
        value: y === "logLine" ? Bu(String(w ?? "")) : w
      },
      y
    )) })
  ] });
}
/*! clipboard-copy. MIT License. Feross Aboukhadijeh <https://feross.org/opensource> */
var oi, Yl;
function Jb() {
  if (Yl) return oi;
  Yl = 1, oi = r;
  function t() {
    return new DOMException("The request is not allowed", "NotAllowedError");
  }
  async function e(n) {
    if (!navigator.clipboard)
      throw t();
    return navigator.clipboard.writeText(n);
  }
  async function a(n) {
    const o = document.createElement("span");
    o.textContent = n, o.style.whiteSpace = "pre", o.style.webkitUserSelect = "auto", o.style.userSelect = "all", document.body.appendChild(o);
    const i = window.getSelection(), s = window.document.createRange();
    i.removeAllRanges(), s.selectNode(o), i.addRange(s);
    let l = !1;
    try {
      l = window.document.execCommand("copy");
    } finally {
      i.removeAllRanges(), window.document.body.removeChild(o);
    }
    if (!l) throw t();
  }
  async function r(n) {
    try {
      await e(n);
    } catch (o) {
      try {
        await a(n);
      } catch (i) {
        throw i || o || t();
      }
    }
  }
  return oi;
}
var Zb = Jb();
const Fu = /* @__PURE__ */ Tr(Zb), gs = ar(
  (t) => fr({
    topicContainer: {
      width: "95%",
      maxWidth: "95%",
      marginTop: t.spacing(2),
      marginBottom: t.spacing(2),
      marginLeft: t.spacing(1),
      borderBottom: 0,
      boxSizing: "border-box"
    },
    topicTypeContainer: {
      height: t.spacing(3),
      display: "flex"
    },
    typeChipContainer: {
      display: "flex",
      height: t.spacing(3),
      marginRight: t.spacing(1),
      alignItems: "center"
    },
    endpointContainer: {
      display: "flex",
      width: "100%",
      maxWidth: "100%",
      padding: t.spacing(2),
      border: 1,
      borderBottom: `1px solid ${t.palette.grey[200]}`,
      gap: t.spacing(1),
      alignItems: "center",
      boxSizing: "border-box",
      overflow: "hidden"
    },
    textInput: {
      flex: 1,
      minWidth: 200,
      backgroundColor: "#ffffff",
      height: 40,
      display: "flex",
      alignItems: "center",
      borderRadius: t.spacing(0.5),
      border: `1px solid ${t.palette.grey[200]}`,
      boxShadow: "0 1px 3px rgba(0, 0, 0, 0.08)",
      padding: t.spacing(0, 1),
      "&:hover": {
        borderColor: t.palette.primary.light
      }
    },
    payloadText: {
      borderTop: 1,
      height: t.spacing(5),
      marginLeft: t.spacing(1)
    },
    tabs: {
      padding: t.spacing(1),
      paddingLeft: t.spacing(1),
      width: "100%",
      maxWidth: "100%",
      boxSizing: "border-box"
    },
    tabPanelContent: {
      width: "100%",
      maxWidth: "100%",
      boxSizing: "border-box"
    },
    payloadContainer: {
      display: "flex",
      flexDirection: "column",
      minHeight: 170,
      height: "auto",
      borderRadius: "none",
      marginBottom: t.spacing(2),
      width: "100%",
      maxWidth: "100%",
      boxSizing: "border-box",
      position: "relative"
    },
    sendIcon: {
      marginLeft: t.spacing(0),
      marginBottom: t.spacing(2),
      width: "100%",
      maxWidth: "100%",
      boxSizing: "border-box"
    },
    parameterContainerWrapper: {
      width: "100%",
      maxWidth: "100%",
      height: t.spacing(27.5),
      overflow: "scroll",
      boxSizing: "border-box"
    },
    label: {
      color: t.palette.secondary.dark,
      fontWeight: 400,
      minWidth: t.spacing(12.5),
      marginTop: "10px"
    },
    textArea: {
      borderRadius: 5,
      "& .MuiOutlinedInput-root": {
        borderRadius: 5,
        padding: 0,
        "& .MuiOutlinedInput-input": {
          backgroundColor: t.palette.common.white,
          borderRadius: 5,
          boxShadow: "inset 0 0 0 1px rgba(85,103,213,0.4), inset 0 1px 1px 0 rgba(0,0,0,0.07), 0 0 0 0 rgba(50,50,77,0.07)",
          padding: t.spacing(2.0375),
          "&:active, &:focus": {
            borderRadius: 5,
            borderColor: "transparent",
            boxShadow: "inset 0 0 0 1px rgba(85,103,213,0.4), inset 0 1px 1px 0 rgba(0,0,0,0.07), 0 0 0 0 rgba(50,50,77,0.07)"
          },
          overflowY: "auto",
          "&::-webkit-scrollbar": {
            width: 4
          },
          "&::-webkit-scrollbar-thumb": {
            background: "#e6e7ec",
            borderRadius: 2
          },
          "&::-webkit-scrollbar-track": {
            backgroundColor: "transparent"
          },
          "&::-webkit-scrollbar-thumb:hover": {
            background: "#eee"
          }
        }
      }
    },
    parameterContainer: {
      display: "grid",
      gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
      gap: t.spacing(1),
      width: "100%",
      maxWidth: "100%",
      height: t.spacing(22),
      borderRadius: "none",
      overflow: "scroll",
      boxSizing: "border-box"
    },
    paramTextInput: {
      size: "small",
      width: "100%",
      maxWidth: "100%"
    },
    apiTokenTextInput: {
      size: "small",
      width: "100%",
      maxWidth: "100%"
    },
    callbackURLTextInput: {
      size: "small",
      width: "100%",
      maxWidth: "100%"
    },
    executeButton: {
      marginLeft: t.spacing(0),
      marginBottom: t.spacing(1),
      width: "100%",
      maxWidth: "100%",
      boxSizing: "border-box"
    },
    outputConsoleContainer: {
      display: "flex",
      justifyContent: "space-between",
      padding: t.spacing(1),
      paddingLeft: t.spacing(2),
      paddingRight: t.spacing(2),
      alignItems: "center",
      borderTop: `1px solid ${t.palette.grey[200]}`,
      width: "100%",
      maxWidth: "100%",
      boxSizing: "border-box"
    },
    outputContainer: {
      height: t.spacing(20),
      overflow: "scroll",
      borderTop: `1px solid ${t.palette.grey[200]}`,
      borderBottomRightRadius: t.spacing(1),
      borderBottomLeftRadius: t.spacing(1),
      width: "100%",
      maxWidth: "100%",
      boxSizing: "border-box",
      paddingLeft: t.spacing(1),
      paddingRight: t.spacing(1)
    },
    logColor1: {
      color: "#745BCC"
    },
    logColor2: {
      color: "#2366CC"
    },
    logColor3: {
      color: "#2366CC"
    },
    logColor4: {
      color: "#0095ff"
    },
    logColor5: {
      color: "#f8c2c2"
    },
    editorContainer: {
      display: "flex",
      width: "100%",
      height: 170,
      position: "relative",
      "& .react-monaco-editor-container": {
        width: "100%",
        height: "100%",
        "& .monaco-editor": {
          "& .monaco-scrollable-element": {
            left: "0 !important"
          }
        }
      }
    },
    copyButton: {
      marginRight: t.spacing(1)
    }
  })
), Qb = ar(
  (t) => fr({
    commons: {
      boxShadow: `0 1px 2px  ${Ye(t.palette.common.black, 0.15)}`,
      borderRadius: 5,
      color: t.palette.common.white,
      padding: t.spacing(0.875, 2),
      gap: t.spacing(1),
      fontWeight: 400,
      fontSize: t.spacing(1.625),
      lineHeight: `${t.spacing(3)}px`,
      "&$disabled": {
        color: t.palette.common.white
      },
      "& $startIcon": {
        "& *:first-child": {
          fontSize: t.spacing(2)
        },
        "&$startIconSmall": {
          "& *:first-child": {
            fontSize: t.spacing(1.75)
          }
        },
        "&$startIconTiny": {
          "& *:first-child": {
            fontSize: t.spacing(1.5)
          }
        }
      },
      "& $endIcon": {
        "& *:first-child": {
          fontSize: t.spacing(2)
        },
        "&$endIconSmall": {
          "& *:first-child": {
            fontSize: t.spacing(1.75)
          }
        },
        "&$endIconTiny": {
          "& *:first-child": {
            fontSize: t.spacing(1.5)
          }
        }
      }
    },
    contained: {
      border: "1px solid transparent",
      "&:hover": {
        boxShadow: `0 1px 3px ${Ye(t.palette.common.black, 0.1)}`
      },
      "&:focus": {
        boxShadow: `0 1px 6px 2px ${Ye(t.palette.common.black, 0.1)}`
      }
    },
    outlined: {
      backgroundColor: "transparent",
      boxShadow: `0 1px 2px ${Ye(t.palette.common.black, 0.05)}`,
      "&:hover, &:focus": {
        backgroundColor: "transparent",
        boxShadow: `0 1px 6px 2px ${Ye(t.palette.common.black, 0.1)}`
      }
    },
    fullWidth: { width: "100%" },
    text: {
      backgroundColor: "transparent",
      border: "none",
      boxShadow: "none"
    },
    subtle: {
      border: `1px solid ${t.palette.grey[100]}`,
      boxShadow: `0 1px 3px ${Ye(t.palette.common.black, 0.05)}`,
      backgroundColor: "#E5E4E2",
      "&:hover": {
        backgroundColor: "#E5E4E2",
        boxShadow: `0 1px 3px ${Ye(t.palette.common.black, 0.1)}`
      },
      "&:focus": {
        backgroundColor: "#E5E4E2",
        boxShadow: "none"
      }
    },
    link: {
      borderColor: "transparent",
      boxShadow: "none",
      paddingLeft: 0,
      paddingRight: 0,
      minWidth: "initial",
      backgroundColor: "transparent",
      "& $startIcon": {
        marginLeft: 0
      },
      "& $endIcon": {
        marginRight: 0
      },
      "&:hover": {
        opacity: 0.6
      },
      "&:hover, &:focus": {
        backgroundColor: "transparent",
        boxShadow: "none"
      }
    },
    primaryContained: {
      backgroundColor: t.palette.primary.main,
      borderColor: t.palette.primary.main,
      "&:hover": {
        backgroundColor: t.palette.primary.dark,
        borderColor: t.palette.primary.dark
      }
    },
    primaryText: {
      color: t.palette.primary.main,
      "&$disabled": {
        color: t.palette.primary.main
      }
    },
    primaryOutlined: {
      color: t.palette.primary.main,
      border: `1px solid ${t.palette.primary.main}`,
      boxShadow: `0 1px 2px  ${Ye(t.palette.common.black, 0.05)}`,
      "&$disabled": {
        color: t.palette.primary.main
      }
    },
    primarySubtle: {
      color: t.palette.primary.main,
      "&$disabled": {
        color: t.palette.primary.main
      }
    },
    primaryLink: {
      color: t.palette.primary.main,
      borderColor: "transparent",
      boxShadow: "none",
      "&$disabled": {
        color: t.palette.primary.main,
        borderColor: "transparent",
        boxShadow: "none"
      }
    },
    secondaryContained: {
      backgroundColor: "#E5E4E2",
      color: t.palette.common.black,
      border: `1px solid ${t.palette.grey[100]}`,
      boxShadow: `0 1px 2px ${Ye(t.palette.common.black, 0.05)}`,
      "&:hover": {
        backgroundColor: "#E5E4E2",
        color: t.palette.common.black
      },
      "&$disabled": {
        color: t.palette.common.black
      }
    },
    secondaryText: {
      color: t.palette.common.black,
      "&$disabled": {
        color: t.palette.common.black
      }
    },
    secondaryOutlined: {
      color: t.palette.secondary.main,
      border: `1px solid ${t.palette.secondary.main}`,
      "&:hover": {
        borderColor: t.palette.secondary.dark
      },
      "&$disabled": {
        color: t.palette.secondary.main
      }
    },
    secondarySubtle: {
      color: t.palette.common.black,
      "&$disabled": {
        color: t.palette.common.black
      }
    },
    secondaryLink: {
      color: t.palette.common.black,
      borderColor: "transparent",
      boxShadow: "none",
      "&$disabled": {
        color: t.palette.common.black,
        borderColor: "transparent",
        boxShadow: "none"
      }
    },
    errorContained: {
      backgroundColor: t.palette.error.main,
      borderColor: t.palette.error.main,
      "&:hover": {
        backgroundColor: t.palette.error.dark,
        borderColor: t.palette.error.dark
      }
    },
    errorOutlined: {
      color: t.palette.error.main,
      border: `1px solid ${t.palette.error.main}`,
      "&:hover": {
        borderColor: t.palette.error.dark
      },
      "&$disabled": {
        color: t.palette.error.main
      }
    },
    errorText: {
      color: t.palette.error.main,
      "&$disabled": {
        color: t.palette.error.main
      }
    },
    errorSubtle: {
      color: t.palette.error.main,
      "&$disabled": {
        color: t.palette.error.main
      }
    },
    errorLink: {
      color: t.palette.error.main,
      borderColor: "transparent",
      boxShadow: "none",
      "&$disabled": {
        color: t.palette.error.main,
        borderColor: "transparent",
        boxShadow: "none"
      }
    },
    successContained: {
      backgroundColor: t.palette.success.main,
      borderColor: t.palette.success.main,
      "&:hover": {
        backgroundColor: t.palette.success.dark,
        borderColor: t.palette.success.dark
      }
    },
    successOutlined: {
      color: t.palette.success.main,
      border: `1px solid ${t.palette.success.main}`,
      "&:hover": {
        borderColor: t.palette.success.dark
      },
      "&$disabled": {
        color: t.palette.success.main
      }
    },
    successText: {
      color: t.palette.success.main,
      "&$disabled": {
        color: t.palette.success.main
      }
    },
    successSubtle: {
      color: t.palette.success.main,
      "&$disabled": {
        color: t.palette.success.main
      }
    },
    successLink: {
      color: t.palette.success.main,
      borderColor: "transparent",
      boxShadow: "none",
      "&$disabled": {
        color: t.palette.success.main,
        borderColor: "transparent",
        boxShadow: "none"
      }
    },
    warningContained: {
      backgroundColor: t.palette.warning.main,
      borderColor: t.palette.warning.main,
      "&:hover": {
        backgroundColor: t.palette.warning.dark,
        borderColor: t.palette.warning.dark
      }
    },
    warningOutlined: {
      color: t.palette.warning.main,
      border: `1px solid ${t.palette.warning.main}`,
      "&:hover": {
        borderColor: t.palette.warning.dark
      },
      "&$disabled": {
        color: t.palette.warning.main
      }
    },
    warningText: {
      color: t.palette.warning.main,
      "&$disabled": {
        color: t.palette.warning.main
      }
    },
    warningSubtle: {
      color: t.palette.warning.main,
      "&$disabled": {
        color: t.palette.warning.main
      }
    },
    warningLink: {
      color: t.palette.warning.main,
      borderColor: "transparent",
      boxShadow: "none",
      "&$disabled": {
        color: t.palette.warning.main,
        borderColor: "transparent",
        boxShadow: "none"
      }
    },
    pill: {
      borderRadius: t.spacing(3.125)
    },
    small: {
      padding: t.spacing(0.375, 2),
      gap: t.spacing(0.75)
    },
    smallPill: {},
    smallLink: {
      padding: t.spacing(0.375, 0)
    },
    tiny: {
      padding: t.spacing(0, 1.5),
      gap: t.spacing(0.5)
    },
    tinyLink: {
      padding: t.spacing(0)
    },
    tinyPill: {},
    disabled: {
      opacity: 0.5,
      cursor: "default",
      pointerEvents: "none",
      color: t.palette.common.white,
      "&:hover": {
        textDecoration: "none"
      }
    },
    startIcon: {},
    startIconSmall: {},
    startIconTiny: {},
    endIcon: {},
    endIconSmall: {},
    endIconTiny: {}
  })
), Br = (t) => {
  const {
    children: e,
    color: a = "primary",
    variant: r = "contained",
    pill: n = !1,
    fullWidth: o = !1,
    size: i = "medium",
    disabled: s = !1,
    testId: l,
    ...u
  } = t, c = Qb(), d = a === "primary", p = a === "secondary", h = a === "error", g = a === "success", m = a === "warning", y = r === "text", w = r === "outlined", k = r === "contained", j = r === "subtle", f = r === "link", x = i === "small", E = i === "tiny";
  return /* @__PURE__ */ B.jsx(
    Pg,
    {
      classes: {
        root: he({
          [c.commons]: !0,
          [c.fullWidth]: o,
          [c.disabled]: s,
          [c.outlined]: w,
          [c.contained]: k,
          [c.text]: y,
          [c.subtle]: j,
          [c.link]: f,
          [c.pill]: n,
          [c.smallPill]: n && x,
          [c.small]: x,
          [c.smallLink]: x && f,
          [c.tinyPill]: n && E,
          [c.tiny]: E,
          [c.tinyLink]: E && f,
          [c.primaryContained]: d && k,
          [c.primaryText]: d && y,
          [c.primaryOutlined]: d && w,
          [c.primarySubtle]: d && j,
          [c.primaryLink]: d && f,
          [c.secondaryContained]: p && k,
          [c.secondaryText]: p && y,
          [c.secondaryOutlined]: p && w,
          [c.secondarySubtle]: p && j,
          [c.secondaryLink]: p && f,
          [c.successContained]: g && k,
          [c.successText]: g && y,
          [c.successOutlined]: g && w,
          [c.successSubtle]: g && j,
          [c.successLink]: g && f,
          [c.errorContained]: h && k,
          [c.errorText]: h && y,
          [c.errorOutlined]: h && w,
          [c.errorSubtle]: h && j,
          [c.errorLink]: h && f,
          [c.warningContained]: m && k,
          [c.warningText]: m && y,
          [c.warningOutlined]: m && w,
          [c.warningSubtle]: m && j,
          [c.warningLink]: m && f
        }),
        startIcon: he({
          [c.startIcon]: !0,
          [c.startIconSmall]: x,
          [c.startIconTiny]: E
        }),
        endIcon: he({
          [c.endIcon]: !0,
          [c.endIconSmall]: x,
          [c.endIconTiny]: E
        })
      },
      disableFocusRipple: !0,
      disableRipple: !0,
      centerRipple: !1,
      "data-cyid": `${l}-button`,
      disabled: s,
      ...u,
      children: e
    }
  );
}, ey = (t) => {
  const e = gs(), { messages: a, clearLogs: r, isDisabled: n } = t, o = () => {
    const i = a.map((s) => `${s.timestamp} ${s.message}`).join(`
`);
    Fu(i);
  };
  return /* @__PURE__ */ B.jsxs(ve, { className: e.outputConsoleContainer, children: [
    /* @__PURE__ */ B.jsx(Mt, { variant: "body1", children: "Output" }),
    /* @__PURE__ */ B.jsxs(ve, { children: [
      /* @__PURE__ */ B.jsx(
        Br,
        {
          testId: "copy-output",
          variant: "link",
          disabled: n,
          onClick: o,
          startIcon: /* @__PURE__ */ B.jsx(Du, {}),
          color: "primary",
          size: "small",
          className: e.copyButton,
          children: "Copy Output"
        }
      ),
      /* @__PURE__ */ B.jsx(
        Br,
        {
          testId: "clear-output",
          variant: "link",
          onClick: r,
          disabled: n,
          startIcon: /* @__PURE__ */ B.jsx(Vb, {}),
          color: "error",
          size: "small",
          children: "Clear"
        }
      )
    ] })
  ] });
}, ty = Ad(() => import("./index-DTebFONq.mjs")), ry = {
  autoIndent: "full",
  automaticLayout: !0,
  contextmenu: !0,
  fontFamily: '"Droid Sans Mono", Monaco, monospace',
  hideCursorInOverviewRuler: !0,
  matchBrackets: "always",
  minimap: {
    enabled: !1
  },
  overviewRulerLanes: 0,
  scrollbar: {
    vertical: "hidden",
    horizontal: "hidden",
    handleMouseWheel: !1
  },
  lineNumbers: !1,
  readOnly: !1,
  renderLineHighlight: "none",
  padding: {
    top: 8
  }
};
function ny({
  fileContent: t,
  language: e,
  width: a = "100%",
  height: r = "70vh",
  setFileContent: n
}) {
  const o = gs();
  return /* @__PURE__ */ B.jsx(ve, { className: o.editorContainer, children: /* @__PURE__ */ B.jsx(fi, { fallback: /* @__PURE__ */ B.jsx(Xn, {}), children: /* @__PURE__ */ B.jsx(
    ty,
    {
      width: a,
      height: r,
      theme: "choreoRequestLightTheme",
      language: e,
      options: ry,
      onChange: (i) => n(i),
      value: t
    }
  ) }) });
}
const zu = ar(
  (t) => fr({
    accordion: {
      "&$accordionBordered": {
        border: `1px solid ${t.palette.grey[100]}`,
        marginBottom: t.spacing(1),
        borderRadius: 8,
        "&.Mui-expanded": {
          marginBottom: t.spacing(1)
        },
        "&:last-child": {
          borderBottomColor: t.palette.grey[100],
          marginBottom: 0
        }
      }
    },
    accordionBordered: {
      "& $accordionSummaryBackgroundFilled": {
        borderRadius: 8
      }
    },
    nestedAccordionContent: {
      width: "100%",
      padding: t.spacing(1, 3)
    },
    accordionSummaryRoot: {
      minHeight: "initial",
      padding: t.spacing(0.5, 2),
      borderBottom: "1px solid transparent",
      "&$accordionSummaryBackgroundFilled": {
        background: "#E5E4E2"
      },
      "&.Mui-expanded": {
        minHeight: "initial",
        borderBottomColor: t.palette.grey[100],
        borderBottomLeftRadius: 0,
        borderBottomRightRadius: 0
      },
      "& .MuiIconButton-label": {
        fontSize: t.spacing(1.5),
        color: t.palette.common.black
      },
      "& .MuiAccordionSummary-content": {
        margin: 0,
        flexGrow: 1,
        overflow: "hidden",
        "&.Mui-expanded": {
          margin: 0
        }
      },
      "&$accordionSummaryNoPaddingRoot": {
        padding: t.spacing(0.5, 0)
      }
    },
    accordionSummaryNoPaddingRoot: {},
    accordionSummaryBackgroundFilled: {}
  })
), ay = ts(cg)(({ theme: t }) => ({
  boxShadow: "none",
  borderBottom: `1px solid ${t.palette.grey[100]}`,
  "&.MuiAccordion-rounded": {
    "&:first-child": {
      borderTopLeftRadius: 8,
      borderTopRightRadius: 8
    },
    "&:last-child": {
      borderBottomLeftRadius: 8,
      borderBottomRightRadius: 8
    }
  },
  "&.MuiPaper-outlined": {
    border: "none",
    borderTop: `1px solid ${t.palette.grey[100]}`,
    backgroundColor: "transparent",
    "&:first-child": {
      borderTop: "none"
    },
    "& .MuiAccordionSummary-root": {
      padding: t.spacing(1, 0)
    }
  },
  "&.Mui-expanded": {
    margin: 0
  },
  "&:before": {
    backgroundColor: "transparent"
  },
  "&:last-child": {
    borderBottomColor: "transparent"
  }
})), Xl = (t) => {
  const { children: e, testId: a, contained: r = !0, bordered: n, ...o } = t, i = r ? void 0 : "outlined", s = zu();
  return /* @__PURE__ */ B.jsx(
    ay,
    {
      "data-cyid": a,
      variant: i,
      ...o,
      classes: {
        root: he({
          [s.accordion]: !0,
          [s.accordionBordered]: n
        })
      },
      children: e
    }
  );
}, oy = ts(fg)(() => ({
  padding: 0
})), Jl = (t) => {
  const { testId: e, children: a, ...r } = t;
  return /* @__PURE__ */ B.jsx(oy, { "data-cyid": e, ...r, children: a });
}, Zl = (t) => {
  const {
    children: e,
    testId: a,
    noPadding: r = !1,
    expandIcon: n,
    backgroundFilled: o = !1,
    className: i,
    ...s
  } = t, l = zu();
  return /* @__PURE__ */ B.jsx(
    kg,
    {
      expandIcon: n || /* @__PURE__ */ B.jsx(Mi, { fontSize: "inherit", rotate: 90 }),
      "data-cyid": a,
      className: he(
        {
          [l.accordionSummaryRoot]: !0,
          [l.accordionSummaryNoPaddingRoot]: r,
          [l.accordionSummaryBackgroundFilled]: o
        },
        i
      ),
      ...s,
      children: e
    }
  );
}, iy = ar(
  (t) => fr({
    root: {
      "& .MuiChip-icon": {
        color: "inherit"
      },
      "&$small": {
        fontSize: t.spacing(1.25),
        borderRadius: t.spacing(0.375),
        lineHeight: 1.2,
        height: t.spacing(2),
        "& .MuiChip-label": {
          padding: t.spacing(0, 0.5, 0.125, 0.5)
        },
        "& .MuiChip-icon": {
          marginLeft: t.spacing(0.5),
          marginRight: 0
        }
      },
      "&$medium": {
        fontSize: t.spacing(1.625),
        borderRadius: t.spacing(0.625),
        lineHeight: 1.23,
        height: t.spacing(3),
        "& .MuiChip-label": {
          padding: t.spacing(0.1, 1)
        },
        "& .MuiChip-icon": {
          marginLeft: t.spacing(1),
          marginRight: 0
        }
      },
      "&$large": {
        fontSize: t.spacing(1.625),
        borderRadius: t.spacing(0.625),
        lineHeight: 1.23,
        "& .MuiChip-label": {
          padding: t.spacing(1, 1.5)
        },
        "& .MuiChip-icon": {
          marginLeft: t.spacing(1.5),
          marginRight: 0
        }
      }
    },
    small: {},
    medium: {},
    large: {},
    contained: {
      "&$info": {
        backgroundColor: "#0095ff",
        color: t.palette.common.white
      },
      "&$primary": {
        backgroundColor: t.palette.primary.main,
        color: t.palette.common.white
      },
      "&$secondary": {
        background: "#E5E4E2"
      },
      "&$success": {
        backgroundColor: t.palette.success.main,
        color: t.palette.common.white
      },
      "&$default": {
        backgroundColor: t.palette.grey[200]
      },
      "&$warning": {
        backgroundColor: t.palette.warning.dark,
        color: t.palette.common.white
      },
      "&$error": {
        backgroundColor: t.palette.error.main,
        color: t.palette.common.white
      }
    },
    outlined: {
      "&$info": {
        color: "#0095ff",
        border: "1px solid 0095ff",
        backgroundColor: Ye("#0095ff", 0.1)
      },
      "&$primary": {
        border: `1px solid ${t.palette.primary.main}`,
        color: t.palette.primary.main,
        backgroundColor: "#D8E1E8"
      },
      "&$secondary": {
        backgroundColor: t.palette.common.white,
        border: `1px solid ${t.palette.grey[200]}`
      },
      "&$success": {
        border: `1px solid ${t.palette.success.main}`,
        color: t.palette.success.main,
        backgroundColor: t.palette.success.light
      },
      "&$default": {
        backgroundColor: t.palette.grey[100],
        border: `1px solid ${t.palette.grey[200]}`
      },
      "&$warning": {
        border: `1px solid ${t.palette.warning.dark}`,
        color: t.palette.warning.dark,
        backgroundColor: t.palette.warning.light
      },
      "&$error": {
        border: `1px solid ${t.palette.error.main}`,
        color: t.palette.error.main,
        backgroundColor: t.palette.error.light
      }
    },
    info: {},
    primary: {},
    secondary: {},
    success: {},
    default: {},
    warning: {},
    error: {}
  })
);
function ii(t) {
  const {
    label: e,
    color: a = "default",
    variant: r = "contained",
    size: n = "medium",
    icon: o,
    testId: i,
    ...s
  } = t, l = iy();
  return /* @__PURE__ */ B.jsx(
    Mg,
    {
      classes: {
        root: he(
          l.root,
          l[n],
          l[r],
          l[a]
        )
      },
      icon: o,
      label: e,
      "data-cyid": `${i}-chip`,
      ...s
    }
  );
}
const Ql = (t) => t, sy = () => {
  let t = Ql;
  return {
    configure(e) {
      t = e;
    },
    generate(e) {
      return t(e);
    },
    reset() {
      t = Ql;
    }
  };
}, ly = sy();
function Kr(t, ...e) {
  const a = new URL(`https://mui.com/production-error/?code=${t}`);
  return e.forEach((r) => a.searchParams.append("args[]", r)), `Minified MUI error #${t}; visit ${a} for the full message.`;
}
function Cn(t) {
  if (typeof t != "string")
    throw new Error(Kr(7));
  return t.charAt(0).toUpperCase() + t.slice(1);
}
function Wu(t) {
  var e, a, r = "";
  if (typeof t == "string" || typeof t == "number") r += t;
  else if (typeof t == "object") if (Array.isArray(t)) {
    var n = t.length;
    for (e = 0; e < n; e++) t[e] && (a = Wu(t[e])) && (r && (r += " "), r += a);
  } else for (a in t) t[a] && (r && (r += " "), r += a);
  return r;
}
function cy() {
  for (var t, e, a = 0, r = "", n = arguments.length; a < n; a++) (t = arguments[a]) && (e = Wu(t)) && (r && (r += " "), r += e);
  return r;
}
function uy(t, e, a = void 0) {
  const r = {};
  for (const n in t) {
    const o = t[n];
    let i = "", s = !0;
    for (let l = 0; l < o.length; l += 1) {
      const u = o[l];
      u && (i += (s === !0 ? "" : " ") + e(u), s = !1, a && a[u] && (i += " " + a[u]));
    }
    r[n] = i;
  }
  return r;
}
var si = { exports: {} }, vt = {};
/**
 * @license React
 * react-is.production.js
 *
 * Copyright (c) Meta Platforms, Inc. and affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
var ec;
function dy() {
  if (ec) return vt;
  ec = 1;
  var t = Symbol.for("react.transitional.element"), e = Symbol.for("react.portal"), a = Symbol.for("react.fragment"), r = Symbol.for("react.strict_mode"), n = Symbol.for("react.profiler"), o = Symbol.for("react.consumer"), i = Symbol.for("react.context"), s = Symbol.for("react.forward_ref"), l = Symbol.for("react.suspense"), u = Symbol.for("react.suspense_list"), c = Symbol.for("react.memo"), d = Symbol.for("react.lazy"), p = Symbol.for("react.view_transition"), h = Symbol.for("react.client.reference");
  function g(m) {
    if (typeof m == "object" && m !== null) {
      var y = m.$$typeof;
      switch (y) {
        case t:
          switch (m = m.type, m) {
            case a:
            case n:
            case r:
            case l:
            case u:
            case p:
              return m;
            default:
              switch (m = m && m.$$typeof, m) {
                case i:
                case s:
                case d:
                case c:
                  return m;
                case o:
                  return m;
                default:
                  return y;
              }
          }
        case e:
          return y;
      }
    }
  }
  return vt.ContextConsumer = o, vt.ContextProvider = i, vt.Element = t, vt.ForwardRef = s, vt.Fragment = a, vt.Lazy = d, vt.Memo = c, vt.Portal = e, vt.Profiler = n, vt.StrictMode = r, vt.Suspense = l, vt.SuspenseList = u, vt.isContextConsumer = function(m) {
    return g(m) === o;
  }, vt.isContextProvider = function(m) {
    return g(m) === i;
  }, vt.isElement = function(m) {
    return typeof m == "object" && m !== null && m.$$typeof === t;
  }, vt.isForwardRef = function(m) {
    return g(m) === s;
  }, vt.isFragment = function(m) {
    return g(m) === a;
  }, vt.isLazy = function(m) {
    return g(m) === d;
  }, vt.isMemo = function(m) {
    return g(m) === c;
  }, vt.isPortal = function(m) {
    return g(m) === e;
  }, vt.isProfiler = function(m) {
    return g(m) === n;
  }, vt.isStrictMode = function(m) {
    return g(m) === r;
  }, vt.isSuspense = function(m) {
    return g(m) === l;
  }, vt.isSuspenseList = function(m) {
    return g(m) === u;
  }, vt.isValidElementType = function(m) {
    return typeof m == "string" || typeof m == "function" || m === a || m === n || m === r || m === l || m === u || typeof m == "object" && m !== null && (m.$$typeof === d || m.$$typeof === c || m.$$typeof === i || m.$$typeof === o || m.$$typeof === s || m.$$typeof === h || m.getModuleId !== void 0);
  }, vt.typeOf = g, vt;
}
var tc;
function fy() {
  return tc || (tc = 1, si.exports = /* @__PURE__ */ dy()), si.exports;
}
var Vu = /* @__PURE__ */ fy();
function lr(t) {
  if (typeof t != "object" || t === null)
    return !1;
  const e = Object.getPrototypeOf(t);
  return (e === null || e === Object.prototype || Object.getPrototypeOf(e) === null) && !(Symbol.toStringTag in t) && !(Symbol.iterator in t);
}
function Hu(t) {
  if (/* @__PURE__ */ b.isValidElement(t) || Vu.isValidElementType(t) || !lr(t))
    return t;
  const e = {};
  return Object.keys(t).forEach((a) => {
    e[a] = Hu(t[a]);
  }), e;
}
function Wt(t, e, a = {
  clone: !0
}) {
  const r = a.clone ? {
    ...t
  } : t;
  return lr(t) && lr(e) && Object.keys(e).forEach((n) => {
    /* @__PURE__ */ b.isValidElement(e[n]) || Vu.isValidElementType(e[n]) ? r[n] = e[n] : lr(e[n]) && // Avoid prototype pollution
    Object.prototype.hasOwnProperty.call(t, n) && lr(t[n]) ? r[n] = Wt(t[n], e[n], a) : a.clone ? r[n] = lr(e[n]) ? Hu(e[n]) : e[n] : r[n] = e[n];
  }), r;
}
function Un(t, e) {
  return e ? Wt(t, e, {
    clone: !1
    // No need to clone deep, it's way faster.
  }) : t;
}
function rc(t, e) {
  if (!t.containerQueries)
    return e;
  const a = Object.keys(e).filter((r) => r.startsWith("@container")).sort((r, n) => {
    var i, s;
    const o = /min-width:\s*([0-9.]+)/;
    return +(((i = r.match(o)) == null ? void 0 : i[1]) || 0) - +(((s = n.match(o)) == null ? void 0 : s[1]) || 0);
  });
  return a.length ? a.reduce((r, n) => {
    const o = e[n];
    return delete r[n], r[n] = o, r;
  }, {
    ...e
  }) : e;
}
function py(t, e) {
  return e === "@" || e.startsWith("@") && (t.some((a) => e.startsWith(`@${a}`)) || !!e.match(/^@\d/));
}
function hy(t, e) {
  const a = e.match(/^@([^/]+)?\/?(.+)?$/);
  if (!a)
    return null;
  const [, r, n] = a, o = Number.isNaN(+r) ? r || 0 : +r;
  return t.containerQueries(n).up(o);
}
function my(t) {
  const e = (o, i) => o.replace("@media", i ? `@container ${i}` : "@container");
  function a(o, i) {
    o.up = (...s) => e(t.breakpoints.up(...s), i), o.down = (...s) => e(t.breakpoints.down(...s), i), o.between = (...s) => e(t.breakpoints.between(...s), i), o.only = (...s) => e(t.breakpoints.only(...s), i), o.not = (...s) => {
      const l = e(t.breakpoints.not(...s), i);
      return l.includes("not all and") ? l.replace("not all and ", "").replace("min-width:", "width<").replace("max-width:", "width>").replace("and", "or") : l;
    };
  }
  const r = {}, n = (o) => (a(r, o), r);
  return a(n), {
    ...t,
    containerQueries: n
  };
}
const oo = {
  xs: 0,
  // phone
  sm: 600,
  // tablet
  md: 900,
  // small laptop
  lg: 1200,
  // desktop
  xl: 1536
  // large screen
}, nc = {
  // Sorted ASC by size. That's important.
  // It can't be configured as it's used statically for propTypes.
  keys: ["xs", "sm", "md", "lg", "xl"],
  up: (t) => `@media (min-width:${oo[t]}px)`
}, gy = {
  containerQueries: (t) => ({
    up: (e) => {
      let a = typeof e == "number" ? e : oo[e] || e;
      return typeof a == "number" && (a = `${a}px`), t ? `@container ${t} (min-width:${a})` : `@container (min-width:${a})`;
    }
  })
};
function ur(t, e, a) {
  const r = t.theme || {};
  if (Array.isArray(e)) {
    const o = r.breakpoints || nc;
    return e.reduce((i, s, l) => (i[o.up(o.keys[l])] = a(e[l]), i), {});
  }
  if (typeof e == "object") {
    const o = r.breakpoints || nc;
    return Object.keys(e).reduce((i, s) => {
      if (py(o.keys, s)) {
        const l = hy(r.containerQueries ? r : gy, s);
        l && (i[l] = a(e[s], s));
      } else if (Object.keys(o.values || oo).includes(s)) {
        const l = o.up(s);
        i[l] = a(e[s], s);
      } else {
        const l = s;
        i[l] = e[l];
      }
      return i;
    }, {});
  }
  return a(e);
}
function vy(t = {}) {
  var a;
  return ((a = t.keys) == null ? void 0 : a.reduce((r, n) => {
    const o = t.up(n);
    return r[o] = {}, r;
  }, {})) || {};
}
function ac(t, e) {
  return t.reduce((a, r) => {
    const n = a[r];
    return (!n || Object.keys(n).length === 0) && delete a[r], a;
  }, e);
}
function io(t, e, a = !0) {
  if (!e || typeof e != "string")
    return null;
  if (t && t.vars && a) {
    const r = `vars.${e}`.split(".").reduce((n, o) => n && n[o] ? n[o] : null, t);
    if (r != null)
      return r;
  }
  return e.split(".").reduce((r, n) => r && r[n] != null ? r[n] : null, t);
}
function Ua(t, e, a, r = a) {
  let n;
  return typeof t == "function" ? n = t(a) : Array.isArray(t) ? n = t[a] || r : n = io(t, a) || r, e && (n = e(n, r, t)), n;
}
function kt(t) {
  const {
    prop: e,
    cssProperty: a = t.prop,
    themeKey: r,
    transform: n
  } = t, o = (i) => {
    if (i[e] == null)
      return null;
    const s = i[e], l = i.theme, u = io(l, r) || {};
    return ur(i, s, (d) => {
      let p = Ua(u, n, d);
      return d === p && typeof d == "string" && (p = Ua(u, n, `${e}${d === "default" ? "" : Cn(d)}`, d)), a === !1 ? p : {
        [a]: p
      };
    });
  };
  return o.propTypes = {}, o.filterProps = [e], o;
}
function by(t) {
  const e = {};
  return (a) => (e[a] === void 0 && (e[a] = t(a)), e[a]);
}
const yy = {
  m: "margin",
  p: "padding"
}, xy = {
  t: "Top",
  r: "Right",
  b: "Bottom",
  l: "Left",
  x: ["Left", "Right"],
  y: ["Top", "Bottom"]
}, oc = {
  marginX: "mx",
  marginY: "my",
  paddingX: "px",
  paddingY: "py"
}, Cy = by((t) => {
  if (t.length > 2)
    if (oc[t])
      t = oc[t];
    else
      return [t];
  const [e, a] = t.split(""), r = yy[e], n = xy[a] || "";
  return Array.isArray(n) ? n.map((o) => r + o) : [r + n];
}), vs = ["m", "mt", "mr", "mb", "ml", "mx", "my", "margin", "marginTop", "marginRight", "marginBottom", "marginLeft", "marginX", "marginY", "marginInline", "marginInlineStart", "marginInlineEnd", "marginBlock", "marginBlockStart", "marginBlockEnd"], bs = ["p", "pt", "pr", "pb", "pl", "px", "py", "padding", "paddingTop", "paddingRight", "paddingBottom", "paddingLeft", "paddingX", "paddingY", "paddingInline", "paddingInlineStart", "paddingInlineEnd", "paddingBlock", "paddingBlockStart", "paddingBlockEnd"];
[...vs, ...bs];
function la(t, e, a, r) {
  const n = io(t, e, !0) ?? a;
  return typeof n == "number" || typeof n == "string" ? (o) => typeof o == "string" ? o : typeof n == "string" ? n.startsWith("var(") && o === 0 ? 0 : n.startsWith("var(") && o === 1 ? n : `calc(${o} * ${n})` : n * o : Array.isArray(n) ? (o) => {
    if (typeof o == "string")
      return o;
    const i = Math.abs(o), s = n[i];
    return o >= 0 ? s : typeof s == "number" ? -s : typeof s == "string" && s.startsWith("var(") ? `calc(-1 * ${s})` : `-${s}`;
  } : typeof n == "function" ? n : () => {
  };
}
function ys(t) {
  return la(t, "spacing", 8);
}
function ca(t, e) {
  return typeof e == "string" || e == null ? e : t(e);
}
function Sy(t, e) {
  return (a) => t.reduce((r, n) => (r[n] = ca(e, a), r), {});
}
function wy(t, e, a, r) {
  if (!e.includes(a))
    return null;
  const n = Cy(a), o = Sy(n, r), i = t[a];
  return ur(t, i, o);
}
function Ku(t, e) {
  const a = ys(t.theme);
  return Object.keys(t).map((r) => wy(t, e, r, a)).reduce(Un, {});
}
function wt(t) {
  return Ku(t, vs);
}
wt.propTypes = {};
wt.filterProps = vs;
function Et(t) {
  return Ku(t, bs);
}
Et.propTypes = {};
Et.filterProps = bs;
function so(...t) {
  const e = t.reduce((r, n) => (n.filterProps.forEach((o) => {
    r[o] = n;
  }), r), {}), a = (r) => Object.keys(r).reduce((n, o) => e[o] ? Un(n, e[o](r)) : n, {});
  return a.propTypes = {}, a.filterProps = t.reduce((r, n) => r.concat(n.filterProps), []), a;
}
function Ht(t) {
  return typeof t != "number" ? t : `${t}px solid`;
}
function qt(t, e) {
  return kt({
    prop: t,
    themeKey: "borders",
    transform: e
  });
}
const Ey = qt("border", Ht), ky = qt("borderTop", Ht), $y = qt("borderRight", Ht), Ry = qt("borderBottom", Ht), Ty = qt("borderLeft", Ht), Oy = qt("borderColor"), _y = qt("borderTopColor"), Py = qt("borderRightColor"), jy = qt("borderBottomColor"), Iy = qt("borderLeftColor"), Ay = qt("outline", Ht), My = qt("outlineColor"), lo = (t) => {
  if (t.borderRadius !== void 0 && t.borderRadius !== null) {
    const e = la(t.theme, "shape.borderRadius", 4), a = (r) => ({
      borderRadius: ca(e, r)
    });
    return ur(t, t.borderRadius, a);
  }
  return null;
};
lo.propTypes = {};
lo.filterProps = ["borderRadius"];
so(Ey, ky, $y, Ry, Ty, Oy, _y, Py, jy, Iy, lo, Ay, My);
const co = (t) => {
  if (t.gap !== void 0 && t.gap !== null) {
    const e = la(t.theme, "spacing", 8), a = (r) => ({
      gap: ca(e, r)
    });
    return ur(t, t.gap, a);
  }
  return null;
};
co.propTypes = {};
co.filterProps = ["gap"];
const uo = (t) => {
  if (t.columnGap !== void 0 && t.columnGap !== null) {
    const e = la(t.theme, "spacing", 8), a = (r) => ({
      columnGap: ca(e, r)
    });
    return ur(t, t.columnGap, a);
  }
  return null;
};
uo.propTypes = {};
uo.filterProps = ["columnGap"];
const fo = (t) => {
  if (t.rowGap !== void 0 && t.rowGap !== null) {
    const e = la(t.theme, "spacing", 8), a = (r) => ({
      rowGap: ca(e, r)
    });
    return ur(t, t.rowGap, a);
  }
  return null;
};
fo.propTypes = {};
fo.filterProps = ["rowGap"];
const Ny = kt({
  prop: "gridColumn"
}), Ly = kt({
  prop: "gridRow"
}), Dy = kt({
  prop: "gridAutoFlow"
}), By = kt({
  prop: "gridAutoColumns"
}), Fy = kt({
  prop: "gridAutoRows"
}), zy = kt({
  prop: "gridTemplateColumns"
}), Wy = kt({
  prop: "gridTemplateRows"
}), Vy = kt({
  prop: "gridTemplateAreas"
}), Hy = kt({
  prop: "gridArea"
});
so(co, uo, fo, Ny, Ly, Dy, By, Fy, zy, Wy, Vy, Hy);
function mn(t, e) {
  return e === "grey" ? e : t;
}
const Ky = kt({
  prop: "color",
  themeKey: "palette",
  transform: mn
}), Uy = kt({
  prop: "bgcolor",
  cssProperty: "backgroundColor",
  themeKey: "palette",
  transform: mn
}), qy = kt({
  prop: "backgroundColor",
  themeKey: "palette",
  transform: mn
});
so(Ky, Uy, qy);
function zt(t) {
  return t <= 1 && t !== 0 ? `${t * 100}%` : t;
}
const Gy = kt({
  prop: "width",
  transform: zt
}), xs = (t) => {
  if (t.maxWidth !== void 0 && t.maxWidth !== null) {
    const e = (a) => {
      var n, o, i, s, l;
      const r = ((i = (o = (n = t.theme) == null ? void 0 : n.breakpoints) == null ? void 0 : o.values) == null ? void 0 : i[a]) || oo[a];
      return r ? ((l = (s = t.theme) == null ? void 0 : s.breakpoints) == null ? void 0 : l.unit) !== "px" ? {
        maxWidth: `${r}${t.theme.breakpoints.unit}`
      } : {
        maxWidth: r
      } : {
        maxWidth: zt(a)
      };
    };
    return ur(t, t.maxWidth, e);
  }
  return null;
};
xs.filterProps = ["maxWidth"];
const Yy = kt({
  prop: "minWidth",
  transform: zt
}), Xy = kt({
  prop: "height",
  transform: zt
}), Jy = kt({
  prop: "maxHeight",
  transform: zt
}), Zy = kt({
  prop: "minHeight",
  transform: zt
});
kt({
  prop: "size",
  cssProperty: "width",
  transform: zt
});
kt({
  prop: "size",
  cssProperty: "height",
  transform: zt
});
const Qy = kt({
  prop: "boxSizing"
});
so(Gy, xs, Yy, Xy, Jy, Zy, Qy);
const po = {
  // borders
  border: {
    themeKey: "borders",
    transform: Ht
  },
  borderTop: {
    themeKey: "borders",
    transform: Ht
  },
  borderRight: {
    themeKey: "borders",
    transform: Ht
  },
  borderBottom: {
    themeKey: "borders",
    transform: Ht
  },
  borderLeft: {
    themeKey: "borders",
    transform: Ht
  },
  borderColor: {
    themeKey: "palette"
  },
  borderTopColor: {
    themeKey: "palette"
  },
  borderRightColor: {
    themeKey: "palette"
  },
  borderBottomColor: {
    themeKey: "palette"
  },
  borderLeftColor: {
    themeKey: "palette"
  },
  outline: {
    themeKey: "borders",
    transform: Ht
  },
  outlineColor: {
    themeKey: "palette"
  },
  borderRadius: {
    themeKey: "shape.borderRadius",
    style: lo
  },
  // palette
  color: {
    themeKey: "palette",
    transform: mn
  },
  bgcolor: {
    themeKey: "palette",
    cssProperty: "backgroundColor",
    transform: mn
  },
  backgroundColor: {
    themeKey: "palette",
    transform: mn
  },
  // spacing
  p: {
    style: Et
  },
  pt: {
    style: Et
  },
  pr: {
    style: Et
  },
  pb: {
    style: Et
  },
  pl: {
    style: Et
  },
  px: {
    style: Et
  },
  py: {
    style: Et
  },
  padding: {
    style: Et
  },
  paddingTop: {
    style: Et
  },
  paddingRight: {
    style: Et
  },
  paddingBottom: {
    style: Et
  },
  paddingLeft: {
    style: Et
  },
  paddingX: {
    style: Et
  },
  paddingY: {
    style: Et
  },
  paddingInline: {
    style: Et
  },
  paddingInlineStart: {
    style: Et
  },
  paddingInlineEnd: {
    style: Et
  },
  paddingBlock: {
    style: Et
  },
  paddingBlockStart: {
    style: Et
  },
  paddingBlockEnd: {
    style: Et
  },
  m: {
    style: wt
  },
  mt: {
    style: wt
  },
  mr: {
    style: wt
  },
  mb: {
    style: wt
  },
  ml: {
    style: wt
  },
  mx: {
    style: wt
  },
  my: {
    style: wt
  },
  margin: {
    style: wt
  },
  marginTop: {
    style: wt
  },
  marginRight: {
    style: wt
  },
  marginBottom: {
    style: wt
  },
  marginLeft: {
    style: wt
  },
  marginX: {
    style: wt
  },
  marginY: {
    style: wt
  },
  marginInline: {
    style: wt
  },
  marginInlineStart: {
    style: wt
  },
  marginInlineEnd: {
    style: wt
  },
  marginBlock: {
    style: wt
  },
  marginBlockStart: {
    style: wt
  },
  marginBlockEnd: {
    style: wt
  },
  // display
  displayPrint: {
    cssProperty: !1,
    transform: (t) => ({
      "@media print": {
        display: t
      }
    })
  },
  display: {},
  overflow: {},
  textOverflow: {},
  visibility: {},
  whiteSpace: {},
  // flexbox
  flexBasis: {},
  flexDirection: {},
  flexWrap: {},
  justifyContent: {},
  alignItems: {},
  alignContent: {},
  order: {},
  flex: {},
  flexGrow: {},
  flexShrink: {},
  alignSelf: {},
  justifyItems: {},
  justifySelf: {},
  // grid
  gap: {
    style: co
  },
  rowGap: {
    style: fo
  },
  columnGap: {
    style: uo
  },
  gridColumn: {},
  gridRow: {},
  gridAutoFlow: {},
  gridAutoColumns: {},
  gridAutoRows: {},
  gridTemplateColumns: {},
  gridTemplateRows: {},
  gridTemplateAreas: {},
  gridArea: {},
  // positions
  position: {},
  zIndex: {
    themeKey: "zIndex"
  },
  top: {},
  right: {},
  bottom: {},
  left: {},
  // shadows
  boxShadow: {
    themeKey: "shadows"
  },
  // sizing
  width: {
    transform: zt
  },
  maxWidth: {
    style: xs
  },
  minWidth: {
    transform: zt
  },
  height: {
    transform: zt
  },
  maxHeight: {
    transform: zt
  },
  minHeight: {
    transform: zt
  },
  boxSizing: {},
  // typography
  font: {
    themeKey: "font"
  },
  fontFamily: {
    themeKey: "typography"
  },
  fontSize: {
    themeKey: "typography"
  },
  fontStyle: {
    themeKey: "typography"
  },
  fontWeight: {
    themeKey: "typography"
  },
  letterSpacing: {},
  textTransform: {},
  lineHeight: {},
  textAlign: {},
  typography: {
    cssProperty: !1,
    themeKey: "typography"
  }
};
function e1(...t) {
  const e = t.reduce((r, n) => r.concat(Object.keys(n)), []), a = new Set(e);
  return t.every((r) => a.size === Object.keys(r).length);
}
function t1(t, e) {
  return typeof t == "function" ? t(e) : t;
}
function r1() {
  function t(a, r, n, o) {
    const i = {
      [a]: r,
      theme: n
    }, s = o[a];
    if (!s)
      return {
        [a]: r
      };
    const {
      cssProperty: l = a,
      themeKey: u,
      transform: c,
      style: d
    } = s;
    if (r == null)
      return null;
    if (u === "typography" && r === "inherit")
      return {
        [a]: r
      };
    const p = io(n, u) || {};
    return d ? d(i) : ur(i, r, (g) => {
      let m = Ua(p, c, g);
      return g === m && typeof g == "string" && (m = Ua(p, c, `${a}${g === "default" ? "" : Cn(g)}`, g)), l === !1 ? m : {
        [l]: m
      };
    });
  }
  function e(a) {
    const {
      sx: r,
      theme: n = {},
      nested: o
    } = a || {};
    if (!r)
      return null;
    const i = n.unstable_sxConfig ?? po;
    function s(l) {
      let u = l;
      if (typeof l == "function")
        u = l(n);
      else if (typeof l != "object")
        return l;
      if (!u)
        return null;
      const c = vy(n.breakpoints), d = Object.keys(c);
      let p = c;
      return Object.keys(u).forEach((h) => {
        const g = t1(u[h], n);
        if (g != null)
          if (typeof g == "object")
            if (i[h])
              p = Un(p, t(h, g, n, i));
            else {
              const m = ur({
                theme: n
              }, g, (y) => ({
                [h]: y
              }));
              e1(m, g) ? p[h] = e({
                sx: g,
                theme: n,
                nested: !0
              }) : p = Un(p, m);
            }
          else
            p = Un(p, t(h, g, n, i));
      }), !o && n.modularCssLayers ? {
        "@layer sx": rc(n, ac(d, p))
      } : rc(n, ac(d, p));
    }
    return Array.isArray(r) ? r.map(s) : s(r);
  }
  return e;
}
const Sn = r1();
Sn.filterProps = ["sx"];
function n1(t) {
  if (t.sheet)
    return t.sheet;
  for (var e = 0; e < document.styleSheets.length; e++)
    if (document.styleSheets[e].ownerNode === t)
      return document.styleSheets[e];
}
function a1(t) {
  var e = document.createElement("style");
  return e.setAttribute("data-emotion", t.key), t.nonce !== void 0 && e.setAttribute("nonce", t.nonce), e.appendChild(document.createTextNode("")), e.setAttribute("data-s", ""), e;
}
var o1 = /* @__PURE__ */ (function() {
  function t(a) {
    var r = this;
    this._insertTag = function(n) {
      var o;
      r.tags.length === 0 ? r.insertionPoint ? o = r.insertionPoint.nextSibling : r.prepend ? o = r.container.firstChild : o = r.before : o = r.tags[r.tags.length - 1].nextSibling, r.container.insertBefore(n, o), r.tags.push(n);
    }, this.isSpeedy = a.speedy === void 0 ? !0 : a.speedy, this.tags = [], this.ctr = 0, this.nonce = a.nonce, this.key = a.key, this.container = a.container, this.prepend = a.prepend, this.insertionPoint = a.insertionPoint, this.before = null;
  }
  var e = t.prototype;
  return e.hydrate = function(r) {
    r.forEach(this._insertTag);
  }, e.insert = function(r) {
    this.ctr % (this.isSpeedy ? 65e3 : 1) === 0 && this._insertTag(a1(this));
    var n = this.tags[this.tags.length - 1];
    if (this.isSpeedy) {
      var o = n1(n);
      try {
        o.insertRule(r, o.cssRules.length);
      } catch {
      }
    } else
      n.appendChild(document.createTextNode(r));
    this.ctr++;
  }, e.flush = function() {
    this.tags.forEach(function(r) {
      var n;
      return (n = r.parentNode) == null ? void 0 : n.removeChild(r);
    }), this.tags = [], this.ctr = 0;
  }, t;
})(), At = "-ms-", qa = "-moz-", it = "-webkit-", Uu = "comm", Cs = "rule", Ss = "decl", i1 = "@import", qu = "@keyframes", s1 = "@layer", l1 = Math.abs, ho = String.fromCharCode, c1 = Object.assign;
function u1(t, e) {
  return It(t, 0) ^ 45 ? (((e << 2 ^ It(t, 0)) << 2 ^ It(t, 1)) << 2 ^ It(t, 2)) << 2 ^ It(t, 3) : 0;
}
function Gu(t) {
  return t.trim();
}
function d1(t, e) {
  return (t = e.exec(t)) ? t[0] : t;
}
function st(t, e, a) {
  return t.replace(e, a);
}
function Ni(t, e) {
  return t.indexOf(e);
}
function It(t, e) {
  return t.charCodeAt(e) | 0;
}
function Jn(t, e, a) {
  return t.slice(e, a);
}
function er(t) {
  return t.length;
}
function ws(t) {
  return t.length;
}
function Sa(t, e) {
  return e.push(t), t;
}
function f1(t, e) {
  return t.map(e).join("");
}
var mo = 1, wn = 1, Yu = 0, Ft = 0, Rt = 0, Tn = "";
function go(t, e, a, r, n, o, i) {
  return { value: t, root: e, parent: a, type: r, props: n, children: o, line: mo, column: wn, length: i, return: "" };
}
function An(t, e) {
  return c1(go("", null, null, "", null, null, 0), t, { length: -t.length }, e);
}
function p1() {
  return Rt;
}
function h1() {
  return Rt = Ft > 0 ? It(Tn, --Ft) : 0, wn--, Rt === 10 && (wn = 1, mo--), Rt;
}
function Vt() {
  return Rt = Ft < Yu ? It(Tn, Ft++) : 0, wn++, Rt === 10 && (wn = 1, mo++), Rt;
}
function nr() {
  return It(Tn, Ft);
}
function Ma() {
  return Ft;
}
function ua(t, e) {
  return Jn(Tn, t, e);
}
function Zn(t) {
  switch (t) {
    // \0 \t \n \r \s whitespace token
    case 0:
    case 9:
    case 10:
    case 13:
    case 32:
      return 5;
    // ! + , / > @ ~ isolate token
    case 33:
    case 43:
    case 44:
    case 47:
    case 62:
    case 64:
    case 126:
    // ; { } breakpoint token
    case 59:
    case 123:
    case 125:
      return 4;
    // : accompanied token
    case 58:
      return 3;
    // " ' ( [ opening delimit token
    case 34:
    case 39:
    case 40:
    case 91:
      return 2;
    // ) ] closing delimit token
    case 41:
    case 93:
      return 1;
  }
  return 0;
}
function Xu(t) {
  return mo = wn = 1, Yu = er(Tn = t), Ft = 0, [];
}
function Ju(t) {
  return Tn = "", t;
}
function Na(t) {
  return Gu(ua(Ft - 1, Li(t === 91 ? t + 2 : t === 40 ? t + 1 : t)));
}
function m1(t) {
  for (; (Rt = nr()) && Rt < 33; )
    Vt();
  return Zn(t) > 2 || Zn(Rt) > 3 ? "" : " ";
}
function g1(t, e) {
  for (; --e && Vt() && !(Rt < 48 || Rt > 102 || Rt > 57 && Rt < 65 || Rt > 70 && Rt < 97); )
    ;
  return ua(t, Ma() + (e < 6 && nr() == 32 && Vt() == 32));
}
function Li(t) {
  for (; Vt(); )
    switch (Rt) {
      // ] ) " '
      case t:
        return Ft;
      // " '
      case 34:
      case 39:
        t !== 34 && t !== 39 && Li(Rt);
        break;
      // (
      case 40:
        t === 41 && Li(t);
        break;
      // \
      case 92:
        Vt();
        break;
    }
  return Ft;
}
function v1(t, e) {
  for (; Vt() && t + Rt !== 57; )
    if (t + Rt === 84 && nr() === 47)
      break;
  return "/*" + ua(e, Ft - 1) + "*" + ho(t === 47 ? t : Vt());
}
function b1(t) {
  for (; !Zn(nr()); )
    Vt();
  return ua(t, Ft);
}
function y1(t) {
  return Ju(La("", null, null, null, [""], t = Xu(t), 0, [0], t));
}
function La(t, e, a, r, n, o, i, s, l) {
  for (var u = 0, c = 0, d = i, p = 0, h = 0, g = 0, m = 1, y = 1, w = 1, k = 0, j = "", f = n, x = o, E = r, L = j; y; )
    switch (g = k, k = Vt()) {
      // (
      case 40:
        if (g != 108 && It(L, d - 1) == 58) {
          Ni(L += st(Na(k), "&", "&\f"), "&\f") != -1 && (w = -1);
          break;
        }
      // " ' [
      case 34:
      case 39:
      case 91:
        L += Na(k);
        break;
      // \t \n \r \s
      case 9:
      case 10:
      case 13:
      case 32:
        L += m1(g);
        break;
      // \
      case 92:
        L += g1(Ma() - 1, 7);
        continue;
      // /
      case 47:
        switch (nr()) {
          case 42:
          case 47:
            Sa(x1(v1(Vt(), Ma()), e, a), l);
            break;
          default:
            L += "/";
        }
        break;
      // {
      case 123 * m:
        s[u++] = er(L) * w;
      // } ; \0
      case 125 * m:
      case 59:
      case 0:
        switch (k) {
          // \0 }
          case 0:
          case 125:
            y = 0;
          // ;
          case 59 + c:
            w == -1 && (L = st(L, /\f/g, "")), h > 0 && er(L) - d && Sa(h > 32 ? sc(L + ";", r, a, d - 1) : sc(st(L, " ", "") + ";", r, a, d - 2), l);
            break;
          // @ ;
          case 59:
            L += ";";
          // { rule/at-rule
          default:
            if (Sa(E = ic(L, e, a, u, c, n, s, j, f = [], x = [], d), o), k === 123)
              if (c === 0)
                La(L, e, E, E, f, o, d, s, x);
              else
                switch (p === 99 && It(L, 3) === 110 ? 100 : p) {
                  // d l m s
                  case 100:
                  case 108:
                  case 109:
                  case 115:
                    La(t, E, E, r && Sa(ic(t, E, E, 0, 0, n, s, j, n, f = [], d), x), n, x, d, s, r ? f : x);
                    break;
                  default:
                    La(L, E, E, E, [""], x, 0, s, x);
                }
        }
        u = c = h = 0, m = w = 1, j = L = "", d = i;
        break;
      // :
      case 58:
        d = 1 + er(L), h = g;
      default:
        if (m < 1) {
          if (k == 123)
            --m;
          else if (k == 125 && m++ == 0 && h1() == 125)
            continue;
        }
        switch (L += ho(k), k * m) {
          // &
          case 38:
            w = c > 0 ? 1 : (L += "\f", -1);
            break;
          // ,
          case 44:
            s[u++] = (er(L) - 1) * w, w = 1;
            break;
          // @
          case 64:
            nr() === 45 && (L += Na(Vt())), p = nr(), c = d = er(j = L += b1(Ma())), k++;
            break;
          // -
          case 45:
            g === 45 && er(L) == 2 && (m = 0);
        }
    }
  return o;
}
function ic(t, e, a, r, n, o, i, s, l, u, c) {
  for (var d = n - 1, p = n === 0 ? o : [""], h = ws(p), g = 0, m = 0, y = 0; g < r; ++g)
    for (var w = 0, k = Jn(t, d + 1, d = l1(m = i[g])), j = t; w < h; ++w)
      (j = Gu(m > 0 ? p[w] + " " + k : st(k, /&\f/g, p[w]))) && (l[y++] = j);
  return go(t, e, a, n === 0 ? Cs : s, l, u, c);
}
function x1(t, e, a) {
  return go(t, e, a, Uu, ho(p1()), Jn(t, 2, -2), 0);
}
function sc(t, e, a, r) {
  return go(t, e, a, Ss, Jn(t, 0, r), Jn(t, r + 1, -1), r);
}
function gn(t, e) {
  for (var a = "", r = ws(t), n = 0; n < r; n++)
    a += e(t[n], n, t, e) || "";
  return a;
}
function C1(t, e, a, r) {
  switch (t.type) {
    case s1:
      if (t.children.length) break;
    case i1:
    case Ss:
      return t.return = t.return || t.value;
    case Uu:
      return "";
    case qu:
      return t.return = t.value + "{" + gn(t.children, r) + "}";
    case Cs:
      t.value = t.props.join(",");
  }
  return er(a = gn(t.children, r)) ? t.return = t.value + "{" + a + "}" : "";
}
function S1(t) {
  var e = ws(t);
  return function(a, r, n, o) {
    for (var i = "", s = 0; s < e; s++)
      i += t[s](a, r, n, o) || "";
    return i;
  };
}
function w1(t) {
  return function(e) {
    e.root || (e = e.return) && t(e);
  };
}
function Zu(t) {
  var e = /* @__PURE__ */ Object.create(null);
  return function(a) {
    return e[a] === void 0 && (e[a] = t(a)), e[a];
  };
}
var E1 = function(e, a, r) {
  for (var n = 0, o = 0; n = o, o = nr(), n === 38 && o === 12 && (a[r] = 1), !Zn(o); )
    Vt();
  return ua(e, Ft);
}, k1 = function(e, a) {
  var r = -1, n = 44;
  do
    switch (Zn(n)) {
      case 0:
        n === 38 && nr() === 12 && (a[r] = 1), e[r] += E1(Ft - 1, a, r);
        break;
      case 2:
        e[r] += Na(n);
        break;
      case 4:
        if (n === 44) {
          e[++r] = nr() === 58 ? "&\f" : "", a[r] = e[r].length;
          break;
        }
      // fallthrough
      default:
        e[r] += ho(n);
    }
  while (n = Vt());
  return e;
}, $1 = function(e, a) {
  return Ju(k1(Xu(e), a));
}, lc = /* @__PURE__ */ new WeakMap(), R1 = function(e) {
  if (!(e.type !== "rule" || !e.parent || // positive .length indicates that this rule contains pseudo
  // negative .length indicates that this rule has been already prefixed
  e.length < 1)) {
    for (var a = e.value, r = e.parent, n = e.column === r.column && e.line === r.line; r.type !== "rule"; )
      if (r = r.parent, !r) return;
    if (!(e.props.length === 1 && a.charCodeAt(0) !== 58 && !lc.get(r)) && !n) {
      lc.set(e, !0);
      for (var o = [], i = $1(a, o), s = r.props, l = 0, u = 0; l < i.length; l++)
        for (var c = 0; c < s.length; c++, u++)
          e.props[u] = o[l] ? i[l].replace(/&\f/g, s[c]) : s[c] + " " + i[l];
    }
  }
}, T1 = function(e) {
  if (e.type === "decl") {
    var a = e.value;
    // charcode for l
    a.charCodeAt(0) === 108 && // charcode for b
    a.charCodeAt(2) === 98 && (e.return = "", e.value = "");
  }
};
function Qu(t, e) {
  switch (u1(t, e)) {
    // color-adjust
    case 5103:
      return it + "print-" + t + t;
    // animation, animation-(delay|direction|duration|fill-mode|iteration-count|name|play-state|timing-function)
    case 5737:
    case 4201:
    case 3177:
    case 3433:
    case 1641:
    case 4457:
    case 2921:
    // text-decoration, filter, clip-path, backface-visibility, column, box-decoration-break
    case 5572:
    case 6356:
    case 5844:
    case 3191:
    case 6645:
    case 3005:
    // mask, mask-image, mask-(mode|clip|size), mask-(repeat|origin), mask-position, mask-composite,
    case 6391:
    case 5879:
    case 5623:
    case 6135:
    case 4599:
    case 4855:
    // background-clip, columns, column-(count|fill|gap|rule|rule-color|rule-style|rule-width|span|width)
    case 4215:
    case 6389:
    case 5109:
    case 5365:
    case 5621:
    case 3829:
      return it + t + t;
    // appearance, user-select, transform, hyphens, text-size-adjust
    case 5349:
    case 4246:
    case 4810:
    case 6968:
    case 2756:
      return it + t + qa + t + At + t + t;
    // flex, flex-direction
    case 6828:
    case 4268:
      return it + t + At + t + t;
    // order
    case 6165:
      return it + t + At + "flex-" + t + t;
    // align-items
    case 5187:
      return it + t + st(t, /(\w+).+(:[^]+)/, it + "box-$1$2" + At + "flex-$1$2") + t;
    // align-self
    case 5443:
      return it + t + At + "flex-item-" + st(t, /flex-|-self/, "") + t;
    // align-content
    case 4675:
      return it + t + At + "flex-line-pack" + st(t, /align-content|flex-|-self/, "") + t;
    // flex-shrink
    case 5548:
      return it + t + At + st(t, "shrink", "negative") + t;
    // flex-basis
    case 5292:
      return it + t + At + st(t, "basis", "preferred-size") + t;
    // flex-grow
    case 6060:
      return it + "box-" + st(t, "-grow", "") + it + t + At + st(t, "grow", "positive") + t;
    // transition
    case 4554:
      return it + st(t, /([^-])(transform)/g, "$1" + it + "$2") + t;
    // cursor
    case 6187:
      return st(st(st(t, /(zoom-|grab)/, it + "$1"), /(image-set)/, it + "$1"), t, "") + t;
    // background, background-image
    case 5495:
    case 3959:
      return st(t, /(image-set\([^]*)/, it + "$1$`$1");
    // justify-content
    case 4968:
      return st(st(t, /(.+:)(flex-)?(.*)/, it + "box-pack:$3" + At + "flex-pack:$3"), /s.+-b[^;]+/, "justify") + it + t + t;
    // (margin|padding)-inline-(start|end)
    case 4095:
    case 3583:
    case 4068:
    case 2532:
      return st(t, /(.+)-inline(.+)/, it + "$1$2") + t;
    // (min|max)?(width|height|inline-size|block-size)
    case 8116:
    case 7059:
    case 5753:
    case 5535:
    case 5445:
    case 5701:
    case 4933:
    case 4677:
    case 5533:
    case 5789:
    case 5021:
    case 4765:
      if (er(t) - 1 - e > 6) switch (It(t, e + 1)) {
        // (m)ax-content, (m)in-content
        case 109:
          if (It(t, e + 4) !== 45) break;
        // (f)ill-available, (f)it-content
        case 102:
          return st(t, /(.+:)(.+)-([^]+)/, "$1" + it + "$2-$3$1" + qa + (It(t, e + 3) == 108 ? "$3" : "$2-$3")) + t;
        // (s)tretch
        case 115:
          return ~Ni(t, "stretch") ? Qu(st(t, "stretch", "fill-available"), e) + t : t;
      }
      break;
    // position: sticky
    case 4949:
      if (It(t, e + 1) !== 115) break;
    // display: (flex|inline-flex)
    case 6444:
      switch (It(t, er(t) - 3 - (~Ni(t, "!important") && 10))) {
        // stic(k)y
        case 107:
          return st(t, ":", ":" + it) + t;
        // (inline-)?fl(e)x
        case 101:
          return st(t, /(.+:)([^;!]+)(;|!.+)?/, "$1" + it + (It(t, 14) === 45 ? "inline-" : "") + "box$3$1" + it + "$2$3$1" + At + "$2box$3") + t;
      }
      break;
    // writing-mode
    case 5936:
      switch (It(t, e + 11)) {
        // vertical-l(r)
        case 114:
          return it + t + At + st(t, /[svh]\w+-[tblr]{2}/, "tb") + t;
        // vertical-r(l)
        case 108:
          return it + t + At + st(t, /[svh]\w+-[tblr]{2}/, "tb-rl") + t;
        // horizontal(-)tb
        case 45:
          return it + t + At + st(t, /[svh]\w+-[tblr]{2}/, "lr") + t;
      }
      return it + t + At + t + t;
  }
  return t;
}
var O1 = function(e, a, r, n) {
  if (e.length > -1 && !e.return) switch (e.type) {
    case Ss:
      e.return = Qu(e.value, e.length);
      break;
    case qu:
      return gn([An(e, {
        value: st(e.value, "@", "@" + it)
      })], n);
    case Cs:
      if (e.length) return f1(e.props, function(o) {
        switch (d1(o, /(::plac\w+|:read-\w+)/)) {
          // :read-(only|write)
          case ":read-only":
          case ":read-write":
            return gn([An(e, {
              props: [st(o, /:(read-\w+)/, ":" + qa + "$1")]
            })], n);
          // :placeholder
          case "::placeholder":
            return gn([An(e, {
              props: [st(o, /:(plac\w+)/, ":" + it + "input-$1")]
            }), An(e, {
              props: [st(o, /:(plac\w+)/, ":" + qa + "$1")]
            }), An(e, {
              props: [st(o, /:(plac\w+)/, At + "input-$1")]
            })], n);
        }
        return "";
      });
  }
}, _1 = [O1], P1 = function(e) {
  var a = e.key;
  if (a === "css") {
    var r = document.querySelectorAll("style[data-emotion]:not([data-s])");
    Array.prototype.forEach.call(r, function(m) {
      var y = m.getAttribute("data-emotion");
      y.indexOf(" ") !== -1 && (document.head.appendChild(m), m.setAttribute("data-s", ""));
    });
  }
  var n = e.stylisPlugins || _1, o = {}, i, s = [];
  i = e.container || document.head, Array.prototype.forEach.call(
    // this means we will ignore elements which don't have a space in them which
    // means that the style elements we're looking at are only Emotion 11 server-rendered style elements
    document.querySelectorAll('style[data-emotion^="' + a + ' "]'),
    function(m) {
      for (var y = m.getAttribute("data-emotion").split(" "), w = 1; w < y.length; w++)
        o[y[w]] = !0;
      s.push(m);
    }
  );
  var l, u = [R1, T1];
  {
    var c, d = [C1, w1(function(m) {
      c.insert(m);
    })], p = S1(u.concat(n, d)), h = function(y) {
      return gn(y1(y), p);
    };
    l = function(y, w, k, j) {
      c = k, h(y ? y + "{" + w.styles + "}" : w.styles), j && (g.inserted[w.name] = !0);
    };
  }
  var g = {
    key: a,
    sheet: new o1({
      key: a,
      container: i,
      nonce: e.nonce,
      speedy: e.speedy,
      prepend: e.prepend,
      insertionPoint: e.insertionPoint
    }),
    nonce: e.nonce,
    inserted: o,
    registered: {},
    insert: l
  };
  return g.sheet.hydrate(s), g;
}, j1 = !0;
function I1(t, e, a) {
  var r = "";
  return a.split(" ").forEach(function(n) {
    t[n] !== void 0 ? e.push(t[n] + ";") : n && (r += n + " ");
  }), r;
}
var ed = function(e, a, r) {
  var n = e.key + "-" + a.name;
  // we only need to add the styles to the registered cache if the
  // class name could be used further down
  // the tree but if it's a string tag, we know it won't
  // so we don't have to add it to registered cache.
  // this improves memory usage since we can avoid storing the whole style string
  (r === !1 || // we need to always store it if we're in compat mode and
  // in node since emotion-server relies on whether a style is in
  // the registered cache to know whether a style is global or not
  // also, note that this check will be dead code eliminated in the browser
  j1 === !1) && e.registered[n] === void 0 && (e.registered[n] = a.styles);
}, A1 = function(e, a, r) {
  ed(e, a, r);
  var n = e.key + "-" + a.name;
  if (e.inserted[a.name] === void 0) {
    var o = a;
    do
      e.insert(a === o ? "." + n : "", o, e.sheet, !0), o = o.next;
    while (o !== void 0);
  }
};
function M1(t) {
  for (var e = 0, a, r = 0, n = t.length; n >= 4; ++r, n -= 4)
    a = t.charCodeAt(r) & 255 | (t.charCodeAt(++r) & 255) << 8 | (t.charCodeAt(++r) & 255) << 16 | (t.charCodeAt(++r) & 255) << 24, a = /* Math.imul(k, m): */
    (a & 65535) * 1540483477 + ((a >>> 16) * 59797 << 16), a ^= /* k >>> r: */
    a >>> 24, e = /* Math.imul(k, m): */
    (a & 65535) * 1540483477 + ((a >>> 16) * 59797 << 16) ^ /* Math.imul(h, m): */
    (e & 65535) * 1540483477 + ((e >>> 16) * 59797 << 16);
  switch (n) {
    case 3:
      e ^= (t.charCodeAt(r + 2) & 255) << 16;
    case 2:
      e ^= (t.charCodeAt(r + 1) & 255) << 8;
    case 1:
      e ^= t.charCodeAt(r) & 255, e = /* Math.imul(h, m): */
      (e & 65535) * 1540483477 + ((e >>> 16) * 59797 << 16);
  }
  return e ^= e >>> 13, e = /* Math.imul(h, m): */
  (e & 65535) * 1540483477 + ((e >>> 16) * 59797 << 16), ((e ^ e >>> 15) >>> 0).toString(36);
}
var N1 = {
  animationIterationCount: 1,
  aspectRatio: 1,
  borderImageOutset: 1,
  borderImageSlice: 1,
  borderImageWidth: 1,
  boxFlex: 1,
  boxFlexGroup: 1,
  boxOrdinalGroup: 1,
  columnCount: 1,
  columns: 1,
  flex: 1,
  flexGrow: 1,
  flexPositive: 1,
  flexShrink: 1,
  flexNegative: 1,
  flexOrder: 1,
  gridRow: 1,
  gridRowEnd: 1,
  gridRowSpan: 1,
  gridRowStart: 1,
  gridColumn: 1,
  gridColumnEnd: 1,
  gridColumnSpan: 1,
  gridColumnStart: 1,
  msGridRow: 1,
  msGridRowSpan: 1,
  msGridColumn: 1,
  msGridColumnSpan: 1,
  fontWeight: 1,
  lineHeight: 1,
  opacity: 1,
  order: 1,
  orphans: 1,
  scale: 1,
  tabSize: 1,
  widows: 1,
  zIndex: 1,
  zoom: 1,
  WebkitLineClamp: 1,
  // SVG-related properties
  fillOpacity: 1,
  floodOpacity: 1,
  stopOpacity: 1,
  strokeDasharray: 1,
  strokeDashoffset: 1,
  strokeMiterlimit: 1,
  strokeOpacity: 1,
  strokeWidth: 1
}, L1 = /[A-Z]|^ms/g, D1 = /_EMO_([^_]+?)_([^]*?)_EMO_/g, td = function(e) {
  return e.charCodeAt(1) === 45;
}, cc = function(e) {
  return e != null && typeof e != "boolean";
}, li = /* @__PURE__ */ Zu(function(t) {
  return td(t) ? t : t.replace(L1, "-$&").toLowerCase();
}), uc = function(e, a) {
  switch (e) {
    case "animation":
    case "animationName":
      if (typeof a == "string")
        return a.replace(D1, function(r, n, o) {
          return tr = {
            name: n,
            styles: o,
            next: tr
          }, n;
        });
  }
  return N1[e] !== 1 && !td(e) && typeof a == "number" && a !== 0 ? a + "px" : a;
};
function Qn(t, e, a) {
  if (a == null)
    return "";
  var r = a;
  if (r.__emotion_styles !== void 0)
    return r;
  switch (typeof a) {
    case "boolean":
      return "";
    case "object": {
      var n = a;
      if (n.anim === 1)
        return tr = {
          name: n.name,
          styles: n.styles,
          next: tr
        }, n.name;
      var o = a;
      if (o.styles !== void 0) {
        var i = o.next;
        if (i !== void 0)
          for (; i !== void 0; )
            tr = {
              name: i.name,
              styles: i.styles,
              next: tr
            }, i = i.next;
        var s = o.styles + ";";
        return s;
      }
      return B1(t, e, a);
    }
    case "function": {
      if (t !== void 0) {
        var l = tr, u = a(t);
        return tr = l, Qn(t, e, u);
      }
      break;
    }
  }
  var c = a;
  if (e == null)
    return c;
  var d = e[c];
  return d !== void 0 ? d : c;
}
function B1(t, e, a) {
  var r = "";
  if (Array.isArray(a))
    for (var n = 0; n < a.length; n++)
      r += Qn(t, e, a[n]) + ";";
  else
    for (var o in a) {
      var i = a[o];
      if (typeof i != "object") {
        var s = i;
        e != null && e[s] !== void 0 ? r += o + "{" + e[s] + "}" : cc(s) && (r += li(o) + ":" + uc(o, s) + ";");
      } else if (Array.isArray(i) && typeof i[0] == "string" && (e == null || e[i[0]] === void 0))
        for (var l = 0; l < i.length; l++)
          cc(i[l]) && (r += li(o) + ":" + uc(o, i[l]) + ";");
      else {
        var u = Qn(t, e, i);
        switch (o) {
          case "animation":
          case "animationName": {
            r += li(o) + ":" + u + ";";
            break;
          }
          default:
            r += o + "{" + u + "}";
        }
      }
    }
  return r;
}
var dc = /label:\s*([^\s;{]+)\s*(;|$)/g, tr;
function rd(t, e, a) {
  if (t.length === 1 && typeof t[0] == "object" && t[0] !== null && t[0].styles !== void 0)
    return t[0];
  var r = !0, n = "";
  tr = void 0;
  var o = t[0];
  if (o == null || o.raw === void 0)
    r = !1, n += Qn(a, e, o);
  else {
    var i = o;
    n += i[0];
  }
  for (var s = 1; s < t.length; s++)
    if (n += Qn(a, e, t[s]), r) {
      var l = o;
      n += l[s];
    }
  dc.lastIndex = 0;
  for (var u = "", c; (c = dc.exec(n)) !== null; )
    u += "-" + c[1];
  var d = M1(n) + u;
  return {
    name: d,
    styles: n,
    next: tr
  };
}
var F1 = function(e) {
  return e();
}, z1 = b.useInsertionEffect ? b.useInsertionEffect : !1, W1 = z1 || F1, nd = /* @__PURE__ */ b.createContext(
  // we're doing this to avoid preconstruct's dead code elimination in this one case
  // because this module is primarily intended for the browser and node
  // but it's also required in react native and similar environments sometimes
  // and we could have a special build just for that
  // but this is much easier and the native packages
  // might use a different theme context in the future anyway
  typeof HTMLElement < "u" ? /* @__PURE__ */ P1({
    key: "css"
  }) : null
);
nd.Provider;
var V1 = function(e) {
  return /* @__PURE__ */ Rc(function(a, r) {
    var n = Md(nd);
    return e(a, n, r);
  });
}, H1 = /* @__PURE__ */ b.createContext({}), K1 = /^((children|dangerouslySetInnerHTML|key|ref|autoFocus|defaultValue|defaultChecked|innerHTML|suppressContentEditableWarning|suppressHydrationWarning|valueLink|abbr|accept|acceptCharset|accessKey|action|allow|allowUserMedia|allowPaymentRequest|allowFullScreen|allowTransparency|alt|async|autoComplete|autoPlay|capture|cellPadding|cellSpacing|challenge|charSet|checked|cite|classID|className|cols|colSpan|content|contentEditable|contextMenu|controls|controlsList|coords|crossOrigin|data|dateTime|decoding|default|defer|dir|disabled|disablePictureInPicture|disableRemotePlayback|download|draggable|encType|enterKeyHint|fetchpriority|fetchPriority|form|formAction|formEncType|formMethod|formNoValidate|formTarget|frameBorder|headers|height|hidden|high|href|hrefLang|htmlFor|httpEquiv|id|inputMode|integrity|is|keyParams|keyType|kind|label|lang|list|loading|loop|low|marginHeight|marginWidth|max|maxLength|media|mediaGroup|method|min|minLength|multiple|muted|name|nonce|noValidate|open|optimum|pattern|placeholder|playsInline|popover|popoverTarget|popoverTargetAction|poster|preload|profile|radioGroup|readOnly|referrerPolicy|rel|required|reversed|role|rows|rowSpan|sandbox|scope|scoped|scrolling|seamless|selected|shape|size|sizes|slot|span|spellCheck|src|srcDoc|srcLang|srcSet|start|step|style|summary|tabIndex|target|title|translate|type|useMap|value|width|wmode|wrap|about|datatype|inlist|prefix|property|resource|typeof|vocab|autoCapitalize|autoCorrect|autoSave|color|incremental|fallback|inert|itemProp|itemScope|itemType|itemID|itemRef|on|option|results|security|unselectable|accentHeight|accumulate|additive|alignmentBaseline|allowReorder|alphabetic|amplitude|arabicForm|ascent|attributeName|attributeType|autoReverse|azimuth|baseFrequency|baselineShift|baseProfile|bbox|begin|bias|by|calcMode|capHeight|clip|clipPathUnits|clipPath|clipRule|colorInterpolation|colorInterpolationFilters|colorProfile|colorRendering|contentScriptType|contentStyleType|cursor|cx|cy|d|decelerate|descent|diffuseConstant|direction|display|divisor|dominantBaseline|dur|dx|dy|edgeMode|elevation|enableBackground|end|exponent|externalResourcesRequired|fill|fillOpacity|fillRule|filter|filterRes|filterUnits|floodColor|floodOpacity|focusable|fontFamily|fontSize|fontSizeAdjust|fontStretch|fontStyle|fontVariant|fontWeight|format|from|fr|fx|fy|g1|g2|glyphName|glyphOrientationHorizontal|glyphOrientationVertical|glyphRef|gradientTransform|gradientUnits|hanging|horizAdvX|horizOriginX|ideographic|imageRendering|in|in2|intercept|k|k1|k2|k3|k4|kernelMatrix|kernelUnitLength|kerning|keyPoints|keySplines|keyTimes|lengthAdjust|letterSpacing|lightingColor|limitingConeAngle|local|markerEnd|markerMid|markerStart|markerHeight|markerUnits|markerWidth|mask|maskContentUnits|maskUnits|mathematical|mode|numOctaves|offset|opacity|operator|order|orient|orientation|origin|overflow|overlinePosition|overlineThickness|panose1|paintOrder|pathLength|patternContentUnits|patternTransform|patternUnits|pointerEvents|points|pointsAtX|pointsAtY|pointsAtZ|preserveAlpha|preserveAspectRatio|primitiveUnits|r|radius|refX|refY|renderingIntent|repeatCount|repeatDur|requiredExtensions|requiredFeatures|restart|result|rotate|rx|ry|scale|seed|shapeRendering|slope|spacing|specularConstant|specularExponent|speed|spreadMethod|startOffset|stdDeviation|stemh|stemv|stitchTiles|stopColor|stopOpacity|strikethroughPosition|strikethroughThickness|string|stroke|strokeDasharray|strokeDashoffset|strokeLinecap|strokeLinejoin|strokeMiterlimit|strokeOpacity|strokeWidth|surfaceScale|systemLanguage|tableValues|targetX|targetY|textAnchor|textDecoration|textRendering|textLength|to|transform|u1|u2|underlinePosition|underlineThickness|unicode|unicodeBidi|unicodeRange|unitsPerEm|vAlphabetic|vHanging|vIdeographic|vMathematical|values|vectorEffect|version|vertAdvY|vertOriginX|vertOriginY|viewBox|viewTarget|visibility|widths|wordSpacing|writingMode|x|xHeight|x1|x2|xChannelSelector|xlinkActuate|xlinkArcrole|xlinkHref|xlinkRole|xlinkShow|xlinkTitle|xlinkType|xmlBase|xmlns|xmlnsXlink|xmlLang|xmlSpace|y|y1|y2|yChannelSelector|z|zoomAndPan|for|class|autofocus)|(([Dd][Aa][Tt][Aa]|[Aa][Rr][Ii][Aa]|x)-.*))$/, U1 = /* @__PURE__ */ Zu(
  function(t) {
    return K1.test(t) || t.charCodeAt(0) === 111 && t.charCodeAt(1) === 110 && t.charCodeAt(2) < 91;
  }
  /* Z+1 */
), q1 = U1, G1 = function(e) {
  return e !== "theme";
}, fc = function(e) {
  return typeof e == "string" && // 96 is one less than the char code
  // for "a" so this is checking that
  // it's a lowercase character
  e.charCodeAt(0) > 96 ? q1 : G1;
}, pc = function(e, a, r) {
  var n;
  if (a) {
    var o = a.shouldForwardProp;
    n = e.__emotion_forwardProp && o ? function(i) {
      return e.__emotion_forwardProp(i) && o(i);
    } : o;
  }
  return typeof n != "function" && r && (n = e.__emotion_forwardProp), n;
}, Y1 = function(e) {
  var a = e.cache, r = e.serialized, n = e.isStringTag;
  return ed(a, r, n), W1(function() {
    return A1(a, r, n);
  }), null;
}, X1 = function t(e, a) {
  var r = e.__emotion_real === e, n = r && e.__emotion_base || e, o, i;
  a !== void 0 && (o = a.label, i = a.target);
  var s = pc(e, a, r), l = s || fc(n), u = !l("as");
  return function() {
    var c = arguments, d = r && e.__emotion_styles !== void 0 ? e.__emotion_styles.slice(0) : [];
    if (o !== void 0 && d.push("label:" + o + ";"), c[0] == null || c[0].raw === void 0)
      d.push.apply(d, c);
    else {
      var p = c[0];
      d.push(p[0]);
      for (var h = c.length, g = 1; g < h; g++)
        d.push(c[g], p[g]);
    }
    var m = V1(function(y, w, k) {
      var j = u && y.as || n, f = "", x = [], E = y;
      if (y.theme == null) {
        E = {};
        for (var L in y)
          E[L] = y[L];
        E.theme = b.useContext(H1);
      }
      typeof y.className == "string" ? f = I1(w.registered, x, y.className) : y.className != null && (f = y.className + " ");
      var W = rd(d.concat(x), w.registered, E);
      f += w.key + "-" + W.name, i !== void 0 && (f += " " + i);
      var O = u && s === void 0 ? fc(j) : l, P = {};
      for (var A in y)
        u && A === "as" || O(A) && (P[A] = y[A]);
      return P.className = f, k && (P.ref = k), /* @__PURE__ */ b.createElement(b.Fragment, null, /* @__PURE__ */ b.createElement(Y1, {
        cache: w,
        serialized: W,
        isStringTag: typeof j == "string"
      }), /* @__PURE__ */ b.createElement(j, P));
    });
    return m.displayName = o !== void 0 ? o : "Styled(" + (typeof n == "string" ? n : n.displayName || n.name || "Component") + ")", m.defaultProps = e.defaultProps, m.__emotion_real = m, m.__emotion_base = n, m.__emotion_styles = d, m.__emotion_forwardProp = s, Object.defineProperty(m, "toString", {
      value: function() {
        return "." + i;
      }
    }), m.withComponent = function(y, w) {
      var k = t(y, Y({}, a, w, {
        shouldForwardProp: pc(m, w, !0)
      }));
      return k.apply(void 0, d);
    }, m;
  };
}, J1 = [
  "a",
  "abbr",
  "address",
  "area",
  "article",
  "aside",
  "audio",
  "b",
  "base",
  "bdi",
  "bdo",
  "big",
  "blockquote",
  "body",
  "br",
  "button",
  "canvas",
  "caption",
  "cite",
  "code",
  "col",
  "colgroup",
  "data",
  "datalist",
  "dd",
  "del",
  "details",
  "dfn",
  "dialog",
  "div",
  "dl",
  "dt",
  "em",
  "embed",
  "fieldset",
  "figcaption",
  "figure",
  "footer",
  "form",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "head",
  "header",
  "hgroup",
  "hr",
  "html",
  "i",
  "iframe",
  "img",
  "input",
  "ins",
  "kbd",
  "keygen",
  "label",
  "legend",
  "li",
  "link",
  "main",
  "map",
  "mark",
  "marquee",
  "menu",
  "menuitem",
  "meta",
  "meter",
  "nav",
  "noscript",
  "object",
  "ol",
  "optgroup",
  "option",
  "output",
  "p",
  "param",
  "picture",
  "pre",
  "progress",
  "q",
  "rp",
  "rt",
  "ruby",
  "s",
  "samp",
  "script",
  "section",
  "select",
  "small",
  "source",
  "span",
  "strong",
  "style",
  "sub",
  "summary",
  "sup",
  "table",
  "tbody",
  "td",
  "textarea",
  "tfoot",
  "th",
  "thead",
  "time",
  "title",
  "tr",
  "track",
  "u",
  "ul",
  "var",
  "video",
  "wbr",
  // SVG
  "circle",
  "clipPath",
  "defs",
  "ellipse",
  "foreignObject",
  "g",
  "image",
  "line",
  "linearGradient",
  "mask",
  "path",
  "pattern",
  "polygon",
  "polyline",
  "radialGradient",
  "rect",
  "stop",
  "svg",
  "text",
  "tspan"
], Di = X1.bind(null);
J1.forEach(function(t) {
  Di[t] = Di(t);
});
function Z1(t, e) {
  return Di(t, e);
}
function Q1(t, e) {
  Array.isArray(t.__emotion_styles) && (t.__emotion_styles = e(t.__emotion_styles));
}
const hc = [];
function Wr(t) {
  return hc[0] = t, rd(hc);
}
const ex = (t) => {
  const e = Object.keys(t).map((a) => ({
    key: a,
    val: t[a]
  })) || [];
  return e.sort((a, r) => a.val - r.val), e.reduce((a, r) => ({
    ...a,
    [r.key]: r.val
  }), {});
};
function tx(t) {
  const {
    // The breakpoint **start** at this value.
    // For instance with the first breakpoint xs: [xs, sm).
    values: e = {
      xs: 0,
      // phone
      sm: 600,
      // tablet
      md: 900,
      // small laptop
      lg: 1200,
      // desktop
      xl: 1536
      // large screen
    },
    unit: a = "px",
    step: r = 5,
    ...n
  } = t, o = ex(e), i = Object.keys(o);
  function s(p) {
    return `@media (min-width:${typeof e[p] == "number" ? e[p] : p}${a})`;
  }
  function l(p) {
    return `@media (max-width:${(typeof e[p] == "number" ? e[p] : p) - r / 100}${a})`;
  }
  function u(p, h) {
    const g = i.indexOf(h);
    return `@media (min-width:${typeof e[p] == "number" ? e[p] : p}${a}) and (max-width:${(g !== -1 && typeof e[i[g]] == "number" ? e[i[g]] : h) - r / 100}${a})`;
  }
  function c(p) {
    return i.indexOf(p) + 1 < i.length ? u(p, i[i.indexOf(p) + 1]) : s(p);
  }
  function d(p) {
    const h = i.indexOf(p);
    return h === 0 ? s(i[1]) : h === i.length - 1 ? l(i[h]) : u(p, i[i.indexOf(p) + 1]).replace("@media", "@media not all and");
  }
  return {
    keys: i,
    values: o,
    up: s,
    down: l,
    between: u,
    only: c,
    not: d,
    unit: a,
    ...n
  };
}
const rx = {
  borderRadius: 4
};
function ad(t = 8, e = ys({
  spacing: t
})) {
  if (t.mui)
    return t;
  const a = (...r) => (r.length === 0 ? [1] : r).map((o) => {
    const i = e(o);
    return typeof i == "number" ? `${i}px` : i;
  }).join(" ");
  return a.mui = !0, a;
}
function nx(t, e) {
  var r;
  const a = this;
  if (a.vars) {
    if (!((r = a.colorSchemes) != null && r[t]) || typeof a.getColorSchemeSelector != "function")
      return {};
    let n = a.getColorSchemeSelector(t);
    return n === "&" ? e : ((n.includes("data-") || n.includes(".")) && (n = `*:where(${n.replace(/\s*&$/, "")}) &`), {
      [n]: e
    });
  }
  return a.palette.mode === t ? e : {};
}
function od(t = {}, ...e) {
  const {
    breakpoints: a = {},
    palette: r = {},
    spacing: n,
    shape: o = {},
    ...i
  } = t, s = tx(a), l = ad(n);
  let u = Wt({
    breakpoints: s,
    direction: "ltr",
    components: {},
    // Inject component definitions.
    palette: {
      mode: "light",
      ...r
    },
    spacing: l,
    shape: {
      ...rx,
      ...o
    }
  }, i);
  return u = my(u), u.applyStyles = nx, u = e.reduce((c, d) => Wt(c, d), u), u.unstable_sxConfig = {
    ...po,
    ...i == null ? void 0 : i.unstable_sxConfig
  }, u.unstable_sx = function(d) {
    return Sn({
      sx: d,
      theme: this
    });
  }, u;
}
const ax = {
  active: "active",
  checked: "checked",
  completed: "completed",
  disabled: "disabled",
  error: "error",
  expanded: "expanded",
  focused: "focused",
  focusVisible: "focusVisible",
  open: "open",
  readOnly: "readOnly",
  required: "required",
  selected: "selected"
};
function id(t, e, a = "Mui") {
  const r = ax[e];
  return r ? `${a}-${r}` : `${ly.generate(t)}-${e}`;
}
function ox(t, e, a = "Mui") {
  const r = {};
  return e.forEach((n) => {
    r[n] = id(t, n, a);
  }), r;
}
function sd(t) {
  const {
    variants: e,
    ...a
  } = t, r = {
    variants: e,
    style: Wr(a),
    isProcessed: !0
  };
  return r.style === a || e && e.forEach((n) => {
    typeof n.style != "function" && (n.style = Wr(n.style));
  }), r;
}
const ix = od();
function ci(t) {
  return t !== "ownerState" && t !== "theme" && t !== "sx" && t !== "as";
}
function Fr(t, e) {
  return e && t && typeof t == "object" && t.styles && !t.styles.startsWith("@layer") && (t.styles = `@layer ${e}{${String(t.styles)}}`), t;
}
function sx(t) {
  return t ? (e, a) => a[t] : null;
}
function lx(t, e, a) {
  t.theme = dx(t.theme) ? a : t.theme[e] || t.theme;
}
function Da(t, e, a) {
  const r = typeof e == "function" ? e(t) : e;
  if (Array.isArray(r))
    return r.flatMap((n) => Da(t, n, a));
  if (Array.isArray(r == null ? void 0 : r.variants)) {
    let n;
    if (r.isProcessed)
      n = a ? Fr(r.style, a) : r.style;
    else {
      const {
        variants: o,
        ...i
      } = r;
      n = a ? Fr(Wr(i), a) : i;
    }
    return ld(t, r.variants, [n], a);
  }
  return r != null && r.isProcessed ? a ? Fr(Wr(r.style), a) : r.style : a ? Fr(Wr(r), a) : r;
}
function ld(t, e, a = [], r = void 0) {
  var o;
  let n;
  e: for (let i = 0; i < e.length; i += 1) {
    const s = e[i];
    if (typeof s.props == "function") {
      if (n ?? (n = {
        ...t,
        ...t.ownerState,
        ownerState: t.ownerState
      }), !s.props(n))
        continue;
    } else
      for (const l in s.props)
        if (t[l] !== s.props[l] && ((o = t.ownerState) == null ? void 0 : o[l]) !== s.props[l])
          continue e;
    typeof s.style == "function" ? (n ?? (n = {
      ...t,
      ...t.ownerState,
      ownerState: t.ownerState
    }), a.push(r ? Fr(Wr(s.style(n)), r) : s.style(n))) : a.push(r ? Fr(Wr(s.style), r) : s.style);
  }
  return a;
}
function cx(t = {}) {
  const {
    themeId: e,
    defaultTheme: a = ix,
    rootShouldForwardProp: r = ci,
    slotShouldForwardProp: n = ci
  } = t;
  function o(s) {
    lx(s, e, a);
  }
  return (s, l = {}) => {
    Q1(s, (E) => E.filter((L) => L !== Sn));
    const {
      name: u,
      slot: c,
      skipVariantsResolver: d,
      skipSx: p,
      // TODO v6: remove `lowercaseFirstLetter()` in the next major release
      // For more details: https://github.com/mui/material-ui/pull/37908
      overridesResolver: h = sx(px(c)),
      ...g
    } = l, m = u && u.startsWith("Mui") || c ? "components" : "custom", y = d !== void 0 ? d : (
      // TODO v6: remove `Root` in the next major release
      // For more details: https://github.com/mui/material-ui/pull/37908
      c && c !== "Root" && c !== "root" || !1
    ), w = p || !1;
    let k = ci;
    c === "Root" || c === "root" ? k = r : c ? k = n : fx(s) && (k = void 0);
    const j = Z1(s, {
      shouldForwardProp: k,
      label: ux(),
      ...g
    }), f = (E) => {
      if (E.__emotion_real === E)
        return E;
      if (typeof E == "function")
        return function(W) {
          return Da(W, E, W.theme.modularCssLayers ? m : void 0);
        };
      if (lr(E)) {
        const L = sd(E);
        return function(O) {
          return L.variants ? Da(O, L, O.theme.modularCssLayers ? m : void 0) : O.theme.modularCssLayers ? Fr(L.style, m) : L.style;
        };
      }
      return E;
    }, x = (...E) => {
      const L = [], W = E.map(f), O = [];
      if (L.push(o), u && h && O.push(function(C) {
        var H, F;
        const D = (F = (H = C.theme.components) == null ? void 0 : H[u]) == null ? void 0 : F.styleOverrides;
        if (!D)
          return null;
        const $ = {};
        for (const q in D)
          $[q] = Da(C, D[q], C.theme.modularCssLayers ? "theme" : void 0);
        return h(C, $);
      }), u && !y && O.push(function(C) {
        var $, H;
        const M = C.theme, D = (H = ($ = M == null ? void 0 : M.components) == null ? void 0 : $[u]) == null ? void 0 : H.variants;
        return D ? ld(C, D, [], C.theme.modularCssLayers ? "theme" : void 0) : null;
      }), w || O.push(Sn), Array.isArray(W[0])) {
        const v = W.shift(), C = new Array(L.length).fill(""), M = new Array(O.length).fill("");
        let D;
        D = [...C, ...v, ...M], D.raw = [...C, ...v.raw, ...M], L.unshift(D);
      }
      const P = [...L, ...W, ...O], A = j(...P);
      return s.muiName && (A.muiName = s.muiName), A;
    };
    return j.withConfig && (x.withConfig = j.withConfig), x;
  };
}
function ux(t, e) {
  return void 0;
}
function dx(t) {
  for (const e in t)
    return !1;
  return !0;
}
function fx(t) {
  return typeof t == "string" && // 96 is one less than the char code
  // for "a" so this is checking that
  // it's a lowercase character
  t.charCodeAt(0) > 96;
}
function px(t) {
  return t && t.charAt(0).toLowerCase() + t.slice(1);
}
function cd(t) {
  var e, a, r = "";
  if (typeof t == "string" || typeof t == "number") r += t;
  else if (typeof t == "object") if (Array.isArray(t)) {
    var n = t.length;
    for (e = 0; e < n; e++) t[e] && (a = cd(t[e])) && (r && (r += " "), r += a);
  } else for (a in t) t[a] && (r && (r += " "), r += a);
  return r;
}
function hx() {
  for (var t, e, a = 0, r = "", n = arguments.length; a < n; a++) (t = arguments[a]) && (e = cd(t)) && (r && (r += " "), r += e);
  return r;
}
function Bi(t, e, a = !1) {
  const r = {
    ...e
  };
  for (const n in t)
    if (Object.prototype.hasOwnProperty.call(t, n)) {
      const o = n;
      if (o === "components" || o === "slots")
        r[o] = {
          ...t[o],
          ...r[o]
        };
      else if (o === "componentsProps" || o === "slotProps") {
        const i = t[o], s = e[o];
        if (!s)
          r[o] = i || {};
        else if (!i)
          r[o] = s;
        else {
          r[o] = {
            ...s
          };
          for (const l in i)
            if (Object.prototype.hasOwnProperty.call(i, l)) {
              const u = l;
              r[o][u] = Bi(i[u], s[u], a);
            }
        }
      } else o === "className" && a && e.className ? r.className = hx(t == null ? void 0 : t.className, e == null ? void 0 : e.className) : o === "style" && a && e.style ? r.style = {
        ...t == null ? void 0 : t.style,
        ...e == null ? void 0 : e.style
      } : r[o] === void 0 && (r[o] = t[o]);
    }
  return r;
}
function mx(t, e = Number.MIN_SAFE_INTEGER, a = Number.MAX_SAFE_INTEGER) {
  return Math.max(e, Math.min(t, a));
}
function Es(t, e = 0, a = 1) {
  return mx(t, e, a);
}
function gx(t) {
  t = t.slice(1);
  const e = new RegExp(`.{1,${t.length >= 6 ? 2 : 1}}`, "g");
  let a = t.match(e);
  return a && a[0].length === 1 && (a = a.map((r) => r + r)), a ? `rgb${a.length === 4 ? "a" : ""}(${a.map((r, n) => n < 3 ? parseInt(r, 16) : Math.round(parseInt(r, 16) / 255 * 1e3) / 1e3).join(", ")})` : "";
}
function Rr(t) {
  if (t.type)
    return t;
  if (t.charAt(0) === "#")
    return Rr(gx(t));
  const e = t.indexOf("("), a = t.substring(0, e);
  if (!["rgb", "rgba", "hsl", "hsla", "color"].includes(a))
    throw new Error(Kr(9, t));
  let r = t.substring(e + 1, t.length - 1), n;
  if (a === "color") {
    if (r = r.split(" "), n = r.shift(), r.length === 4 && r[3].charAt(0) === "/" && (r[3] = r[3].slice(1)), !["srgb", "display-p3", "a98-rgb", "prophoto-rgb", "rec-2020"].includes(n))
      throw new Error(Kr(10, n));
  } else
    r = r.split(",");
  return r = r.map((o) => parseFloat(o)), {
    type: a,
    values: r,
    colorSpace: n
  };
}
const vx = (t) => {
  const e = Rr(t);
  return e.values.slice(0, 3).map((a, r) => e.type.includes("hsl") && r !== 0 ? `${a}%` : a).join(" ");
}, zn = (t, e) => {
  try {
    return vx(t);
  } catch {
    return t;
  }
};
function vo(t) {
  const {
    type: e,
    colorSpace: a
  } = t;
  let {
    values: r
  } = t;
  return e.includes("rgb") ? r = r.map((n, o) => o < 3 ? parseInt(n, 10) : n) : e.includes("hsl") && (r[1] = `${r[1]}%`, r[2] = `${r[2]}%`), e.includes("color") ? r = `${a} ${r.join(" ")}` : r = `${r.join(", ")}`, `${e}(${r})`;
}
function ud(t) {
  t = Rr(t);
  const {
    values: e
  } = t, a = e[0], r = e[1] / 100, n = e[2] / 100, o = r * Math.min(n, 1 - n), i = (u, c = (u + a / 30) % 12) => n - o * Math.max(Math.min(c - 3, 9 - c, 1), -1);
  let s = "rgb";
  const l = [Math.round(i(0) * 255), Math.round(i(8) * 255), Math.round(i(4) * 255)];
  return t.type === "hsla" && (s += "a", l.push(e[3])), vo({
    type: s,
    values: l
  });
}
function Fi(t) {
  t = Rr(t);
  let e = t.type === "hsl" || t.type === "hsla" ? Rr(ud(t)).values : t.values;
  return e = e.map((a) => (t.type !== "color" && (a /= 255), a <= 0.03928 ? a / 12.92 : ((a + 0.055) / 1.055) ** 2.4)), Number((0.2126 * e[0] + 0.7152 * e[1] + 0.0722 * e[2]).toFixed(3));
}
function bx(t, e) {
  const a = Fi(t), r = Fi(e);
  return (Math.max(a, r) + 0.05) / (Math.min(a, r) + 0.05);
}
function dd(t, e) {
  return t = Rr(t), e = Es(e), (t.type === "rgb" || t.type === "hsl") && (t.type += "a"), t.type === "color" ? t.values[3] = `/${e}` : t.values[3] = e, vo(t);
}
function Ar(t, e, a) {
  try {
    return dd(t, e);
  } catch {
    return t;
  }
}
function bo(t, e) {
  if (t = Rr(t), e = Es(e), t.type.includes("hsl"))
    t.values[2] *= 1 - e;
  else if (t.type.includes("rgb") || t.type.includes("color"))
    for (let a = 0; a < 3; a += 1)
      t.values[a] *= 1 - e;
  return vo(t);
}
function dt(t, e, a) {
  try {
    return bo(t, e);
  } catch {
    return t;
  }
}
function yo(t, e) {
  if (t = Rr(t), e = Es(e), t.type.includes("hsl"))
    t.values[2] += (100 - t.values[2]) * e;
  else if (t.type.includes("rgb"))
    for (let a = 0; a < 3; a += 1)
      t.values[a] += (255 - t.values[a]) * e;
  else if (t.type.includes("color"))
    for (let a = 0; a < 3; a += 1)
      t.values[a] += (1 - t.values[a]) * e;
  return vo(t);
}
function ft(t, e, a) {
  try {
    return yo(t, e);
  } catch {
    return t;
  }
}
function yx(t, e = 0.15) {
  return Fi(t) > 0.5 ? bo(t, e) : yo(t, e);
}
function wa(t, e, a) {
  try {
    return yx(t, e);
  } catch {
    return t;
  }
}
const xx = /* @__PURE__ */ b.createContext(void 0);
function Cx(t) {
  const {
    theme: e,
    name: a,
    props: r
  } = t;
  if (!e || !e.components || !e.components[a])
    return r;
  const n = e.components[a];
  return n.defaultProps ? Bi(n.defaultProps, r, e.components.mergeClassNameAndStyle) : !n.styleOverrides && !n.variants ? Bi(n, r, e.components.mergeClassNameAndStyle) : r;
}
function Sx({
  props: t,
  name: e
}) {
  const a = b.useContext(xx);
  return Cx({
    props: t,
    name: e,
    theme: {
      components: a
    }
  });
}
const mc = {
  theme: void 0
};
function wx(t) {
  let e, a;
  return function(n) {
    let o = e;
    return (o === void 0 || n.theme !== a) && (mc.theme = n.theme, o = sd(t(mc)), e = o, a = n.theme), o;
  };
}
function Ex(t = "") {
  function e(...r) {
    if (!r.length)
      return "";
    const n = r[0];
    return typeof n == "string" && !n.match(/(#|\(|\)|(-?(\d*\.)?\d+)(px|em|%|ex|ch|rem|vw|vh|vmin|vmax|cm|mm|in|pt|pc))|^(-?(\d*\.)?\d+)$|(\d+ \d+ \d+)/) ? `, var(--${t ? `${t}-` : ""}${n}${e(...r.slice(1))})` : `, ${n}`;
  }
  return (r, ...n) => `var(--${t ? `${t}-` : ""}${r}${e(...n)})`;
}
const gc = (t, e, a, r = []) => {
  let n = t;
  e.forEach((o, i) => {
    i === e.length - 1 ? Array.isArray(n) ? n[Number(o)] = a : n && typeof n == "object" && (n[o] = a) : n && typeof n == "object" && (n[o] || (n[o] = r.includes(o) ? [] : {}), n = n[o]);
  });
}, kx = (t, e, a) => {
  function r(n, o = [], i = []) {
    Object.entries(n).forEach(([s, l]) => {
      (!a || a && !a([...o, s])) && l != null && (typeof l == "object" && Object.keys(l).length > 0 ? r(l, [...o, s], Array.isArray(l) ? [...i, s] : i) : e([...o, s], l, i));
    });
  }
  r(t);
}, $x = (t, e) => typeof e == "number" ? ["lineHeight", "fontWeight", "opacity", "zIndex"].some((r) => t.includes(r)) || t[t.length - 1].toLowerCase().includes("opacity") ? e : `${e}px` : e;
function ui(t, e) {
  const {
    prefix: a,
    shouldSkipGeneratingVar: r
  } = e || {}, n = {}, o = {}, i = {};
  return kx(
    t,
    (s, l, u) => {
      if ((typeof l == "string" || typeof l == "number") && (!r || !r(s, l))) {
        const c = `--${a ? `${a}-` : ""}${s.join("-")}`, d = $x(s, l);
        Object.assign(n, {
          [c]: d
        }), gc(o, s, `var(${c})`, u), gc(i, s, `var(${c}, ${d})`, u);
      }
    },
    (s) => s[0] === "vars"
    // skip 'vars/*' paths
  ), {
    css: n,
    vars: o,
    varsWithDefaults: i
  };
}
function Rx(t, e = {}) {
  const {
    getSelector: a = w,
    disableCssColorScheme: r,
    colorSchemeSelector: n,
    enableContrastVars: o
  } = e, {
    colorSchemes: i = {},
    components: s,
    defaultColorScheme: l = "light",
    ...u
  } = t, {
    vars: c,
    css: d,
    varsWithDefaults: p
  } = ui(u, e);
  let h = p;
  const g = {}, {
    [l]: m,
    ...y
  } = i;
  if (Object.entries(y || {}).forEach(([f, x]) => {
    const {
      vars: E,
      css: L,
      varsWithDefaults: W
    } = ui(x, e);
    h = Wt(h, W), g[f] = {
      css: L,
      vars: E
    };
  }), m) {
    const {
      css: f,
      vars: x,
      varsWithDefaults: E
    } = ui(m, e);
    h = Wt(h, E), g[l] = {
      css: f,
      vars: x
    };
  }
  function w(f, x) {
    var L, W;
    let E = n;
    if (n === "class" && (E = ".%s"), n === "data" && (E = "[data-%s]"), n != null && n.startsWith("data-") && !n.includes("%s") && (E = `[${n}="%s"]`), f) {
      if (E === "media")
        return t.defaultColorScheme === f ? ":root" : {
          [`@media (prefers-color-scheme: ${((W = (L = i[f]) == null ? void 0 : L.palette) == null ? void 0 : W.mode) || f})`]: {
            ":root": x
          }
        };
      if (E)
        return t.defaultColorScheme === f ? `:root, ${E.replace("%s", String(f))}` : E.replace("%s", String(f));
    }
    return ":root";
  }
  return {
    vars: h,
    generateThemeVars: () => {
      let f = {
        ...c
      };
      return Object.entries(g).forEach(([, {
        vars: x
      }]) => {
        f = Wt(f, x);
      }), f;
    },
    generateStyleSheets: () => {
      var O, P;
      const f = [], x = t.defaultColorScheme || "light";
      function E(A, v) {
        Object.keys(v).length && f.push(typeof A == "string" ? {
          [A]: {
            ...v
          }
        } : A);
      }
      E(a(void 0, {
        ...d
      }), d);
      const {
        [x]: L,
        ...W
      } = g;
      if (L) {
        const {
          css: A
        } = L, v = (P = (O = i[x]) == null ? void 0 : O.palette) == null ? void 0 : P.mode, C = !r && v ? {
          colorScheme: v,
          ...A
        } : {
          ...A
        };
        E(a(x, {
          ...C
        }), C);
      }
      return Object.entries(W).forEach(([A, {
        css: v
      }]) => {
        var D, $;
        const C = ($ = (D = i[A]) == null ? void 0 : D.palette) == null ? void 0 : $.mode, M = !r && C ? {
          colorScheme: C,
          ...v
        } : {
          ...v
        };
        E(a(A, {
          ...M
        }), M);
      }), o && f.push({
        ":root": {
          // use double underscore to indicate that these are private variables
          "--__l-threshold": "0.7",
          "--__l": "clamp(0, (l / var(--__l-threshold) - 1) * -infinity, 1)",
          "--__a": "clamp(0.87, (l / var(--__l-threshold) - 1) * -infinity, 1)"
          // 0.87 is the default alpha value for black text.
        }
      }), f;
    }
  };
}
function Tx(t) {
  return function(a) {
    return t === "media" ? `@media (prefers-color-scheme: ${a})` : t ? t.startsWith("data-") && !t.includes("%s") ? `[${t}="${a}"] &` : t === "class" ? `.${a} &` : t === "data" ? `[data-${a}] &` : `${t.replace("%s", a)} &` : "&";
  };
}
const ea = {
  black: "#000",
  white: "#fff"
}, Ox = {
  50: "#fafafa",
  100: "#f5f5f5",
  200: "#eeeeee",
  300: "#e0e0e0",
  400: "#bdbdbd",
  500: "#9e9e9e",
  600: "#757575",
  700: "#616161",
  800: "#424242",
  900: "#212121",
  A100: "#f5f5f5",
  A200: "#eeeeee",
  A400: "#bdbdbd",
  A700: "#616161"
}, ln = {
  50: "#f3e5f5",
  200: "#ce93d8",
  300: "#ba68c8",
  400: "#ab47bc",
  500: "#9c27b0",
  700: "#7b1fa2"
}, cn = {
  300: "#e57373",
  400: "#ef5350",
  500: "#f44336",
  700: "#d32f2f",
  800: "#c62828"
}, Mn = {
  300: "#ffb74d",
  400: "#ffa726",
  500: "#ff9800",
  700: "#f57c00",
  900: "#e65100"
}, un = {
  50: "#e3f2fd",
  200: "#90caf9",
  400: "#42a5f5",
  700: "#1976d2",
  800: "#1565c0"
}, dn = {
  300: "#4fc3f7",
  400: "#29b6f6",
  500: "#03a9f4",
  700: "#0288d1",
  900: "#01579b"
}, fn = {
  300: "#81c784",
  400: "#66bb6a",
  500: "#4caf50",
  700: "#388e3c",
  800: "#2e7d32",
  900: "#1b5e20"
};
function fd() {
  return {
    // The colors used to style the text.
    text: {
      // The most important text.
      primary: "rgba(0, 0, 0, 0.87)",
      // Secondary text.
      secondary: "rgba(0, 0, 0, 0.6)",
      // Disabled text have even lower visual prominence.
      disabled: "rgba(0, 0, 0, 0.38)"
    },
    // The color used to divide different elements.
    divider: "rgba(0, 0, 0, 0.12)",
    // The background colors used to style the surfaces.
    // Consistency between these values is important.
    background: {
      paper: ea.white,
      default: ea.white
    },
    // The colors used to style the action elements.
    action: {
      // The color of an active action like an icon button.
      active: "rgba(0, 0, 0, 0.54)",
      // The color of an hovered action.
      hover: "rgba(0, 0, 0, 0.04)",
      hoverOpacity: 0.04,
      // The color of a selected action.
      selected: "rgba(0, 0, 0, 0.08)",
      selectedOpacity: 0.08,
      // The color of a disabled action.
      disabled: "rgba(0, 0, 0, 0.26)",
      // The background color of a disabled action.
      disabledBackground: "rgba(0, 0, 0, 0.12)",
      disabledOpacity: 0.38,
      focus: "rgba(0, 0, 0, 0.12)",
      focusOpacity: 0.12,
      activatedOpacity: 0.12
    }
  };
}
const pd = fd();
function hd() {
  return {
    text: {
      primary: ea.white,
      secondary: "rgba(255, 255, 255, 0.7)",
      disabled: "rgba(255, 255, 255, 0.5)",
      icon: "rgba(255, 255, 255, 0.5)"
    },
    divider: "rgba(255, 255, 255, 0.12)",
    background: {
      paper: "#121212",
      default: "#121212"
    },
    action: {
      active: ea.white,
      hover: "rgba(255, 255, 255, 0.08)",
      hoverOpacity: 0.08,
      selected: "rgba(255, 255, 255, 0.16)",
      selectedOpacity: 0.16,
      disabled: "rgba(255, 255, 255, 0.3)",
      disabledBackground: "rgba(255, 255, 255, 0.12)",
      disabledOpacity: 0.38,
      focus: "rgba(255, 255, 255, 0.12)",
      focusOpacity: 0.12,
      activatedOpacity: 0.24
    }
  };
}
const zi = hd();
function vc(t, e, a, r) {
  const n = r.light || r, o = r.dark || r * 1.5;
  t[e] || (t.hasOwnProperty(a) ? t[e] = t[a] : e === "light" ? t.light = yo(t.main, n) : e === "dark" && (t.dark = bo(t.main, o)));
}
function bc(t, e, a, r, n) {
  const o = n.light || n, i = n.dark || n * 1.5;
  e[a] || (e.hasOwnProperty(r) ? e[a] = e[r] : a === "light" ? e.light = `color-mix(in ${t}, ${e.main}, #fff ${(o * 100).toFixed(0)}%)` : a === "dark" && (e.dark = `color-mix(in ${t}, ${e.main}, #000 ${(i * 100).toFixed(0)}%)`));
}
function _x(t = "light") {
  return t === "dark" ? {
    main: un[200],
    light: un[50],
    dark: un[400]
  } : {
    main: un[700],
    light: un[400],
    dark: un[800]
  };
}
function Px(t = "light") {
  return t === "dark" ? {
    main: ln[200],
    light: ln[50],
    dark: ln[400]
  } : {
    main: ln[500],
    light: ln[300],
    dark: ln[700]
  };
}
function jx(t = "light") {
  return t === "dark" ? {
    main: cn[500],
    light: cn[300],
    dark: cn[700]
  } : {
    main: cn[700],
    light: cn[400],
    dark: cn[800]
  };
}
function Ix(t = "light") {
  return t === "dark" ? {
    main: dn[400],
    light: dn[300],
    dark: dn[700]
  } : {
    main: dn[700],
    light: dn[500],
    dark: dn[900]
  };
}
function Ax(t = "light") {
  return t === "dark" ? {
    main: fn[400],
    light: fn[300],
    dark: fn[700]
  } : {
    main: fn[800],
    light: fn[500],
    dark: fn[900]
  };
}
function Mx(t = "light") {
  return t === "dark" ? {
    main: Mn[400],
    light: Mn[300],
    dark: Mn[700]
  } : {
    main: "#ed6c02",
    // closest to orange[800] that pass 3:1.
    light: Mn[500],
    dark: Mn[900]
  };
}
function Nx(t) {
  return `oklch(from ${t} var(--__l) 0 h / var(--__a))`;
}
function ks(t) {
  const {
    mode: e = "light",
    contrastThreshold: a = 3,
    tonalOffset: r = 0.2,
    colorSpace: n,
    ...o
  } = t, i = t.primary || _x(e), s = t.secondary || Px(e), l = t.error || jx(e), u = t.info || Ix(e), c = t.success || Ax(e), d = t.warning || Mx(e);
  function p(y) {
    return n ? Nx(y) : bx(y, zi.text.primary) >= a ? zi.text.primary : pd.text.primary;
  }
  const h = ({
    color: y,
    name: w,
    mainShade: k = 500,
    lightShade: j = 300,
    darkShade: f = 700
  }) => {
    if (y = {
      ...y
    }, !y.main && y[k] && (y.main = y[k]), !y.hasOwnProperty("main"))
      throw new Error(Kr(11, w ? ` (${w})` : "", k));
    if (typeof y.main != "string")
      throw new Error(Kr(12, w ? ` (${w})` : "", JSON.stringify(y.main)));
    return n ? (bc(n, y, "light", j, r), bc(n, y, "dark", f, r)) : (vc(y, "light", j, r), vc(y, "dark", f, r)), y.contrastText || (y.contrastText = p(y.main)), y;
  };
  let g;
  return e === "light" ? g = fd() : e === "dark" && (g = hd()), Wt({
    // A collection of common colors.
    common: {
      ...ea
    },
    // prevent mutable object.
    // The palette mode, can be light or dark.
    mode: e,
    // The colors used to represent primary interface elements for a user.
    primary: h({
      color: i,
      name: "primary"
    }),
    // The colors used to represent secondary interface elements for a user.
    secondary: h({
      color: s,
      name: "secondary",
      mainShade: "A400",
      lightShade: "A200",
      darkShade: "A700"
    }),
    // The colors used to represent interface elements that the user should be made aware of.
    error: h({
      color: l,
      name: "error"
    }),
    // The colors used to represent potentially dangerous actions or important messages.
    warning: h({
      color: d,
      name: "warning"
    }),
    // The colors used to present information to the user that is neutral and not necessarily important.
    info: h({
      color: u,
      name: "info"
    }),
    // The colors used to indicate the successful completion of an action that user triggered.
    success: h({
      color: c,
      name: "success"
    }),
    // The grey colors.
    grey: Ox,
    // Used by `getContrastText()` to maximize the contrast between
    // the background and the text.
    contrastThreshold: a,
    // Takes a background color and returns the text color that maximizes the contrast.
    getContrastText: p,
    // Generate a rich color object.
    augmentColor: h,
    // Used by the functions below to shift a color's luminance by approximately
    // two indexes within its tonal palette.
    // E.g., shift from Red 500 to Red 300 or Red 700.
    tonalOffset: r,
    // The light and dark mode object.
    ...g
  }, o);
}
function Lx(t) {
  const e = {};
  return Object.entries(t).forEach((r) => {
    const [n, o] = r;
    typeof o == "object" && (e[n] = `${o.fontStyle ? `${o.fontStyle} ` : ""}${o.fontVariant ? `${o.fontVariant} ` : ""}${o.fontWeight ? `${o.fontWeight} ` : ""}${o.fontStretch ? `${o.fontStretch} ` : ""}${o.fontSize || ""}${o.lineHeight ? `/${o.lineHeight} ` : ""}${o.fontFamily || ""}`);
  }), e;
}
function Dx(t, e) {
  return {
    toolbar: {
      minHeight: 56,
      [t.up("xs")]: {
        "@media (orientation: landscape)": {
          minHeight: 48
        }
      },
      [t.up("sm")]: {
        minHeight: 64
      }
    },
    ...e
  };
}
function Bx(t) {
  return Math.round(t * 1e5) / 1e5;
}
const yc = {
  textTransform: "uppercase"
}, xc = '"Roboto", "Helvetica", "Arial", sans-serif';
function Fx(t, e) {
  const {
    fontFamily: a = xc,
    // The default font size of the Material Specification.
    fontSize: r = 14,
    // px
    fontWeightLight: n = 300,
    fontWeightRegular: o = 400,
    fontWeightMedium: i = 500,
    fontWeightBold: s = 700,
    // Tell MUI what's the font-size on the html element.
    // 16px is the default font-size used by browsers.
    htmlFontSize: l = 16,
    // Apply the CSS properties to all the variants.
    allVariants: u,
    pxToRem: c,
    ...d
  } = typeof e == "function" ? e(t) : e, p = r / 14, h = c || ((y) => `${y / l * p}rem`), g = (y, w, k, j, f) => ({
    fontFamily: a,
    fontWeight: y,
    fontSize: h(w),
    // Unitless following https://meyerweb.com/eric/thoughts/2006/02/08/unitless-line-heights/
    lineHeight: k,
    // The letter spacing was designed for the Roboto font-family. Using the same letter-spacing
    // across font-families can cause issues with the kerning.
    ...a === xc ? {
      letterSpacing: `${Bx(j / w)}em`
    } : {},
    ...f,
    ...u
  }), m = {
    h1: g(n, 96, 1.167, -1.5),
    h2: g(n, 60, 1.2, -0.5),
    h3: g(o, 48, 1.167, 0),
    h4: g(o, 34, 1.235, 0.25),
    h5: g(o, 24, 1.334, 0),
    h6: g(i, 20, 1.6, 0.15),
    subtitle1: g(o, 16, 1.75, 0.15),
    subtitle2: g(i, 14, 1.57, 0.1),
    body1: g(o, 16, 1.5, 0.15),
    body2: g(o, 14, 1.43, 0.15),
    button: g(i, 14, 1.75, 0.4, yc),
    caption: g(o, 12, 1.66, 0.4),
    overline: g(o, 12, 2.66, 1, yc),
    // TODO v6: Remove handling of 'inherit' variant from the theme as it is already handled in Material UI's Typography component. Also, remember to remove the associated types.
    inherit: {
      fontFamily: "inherit",
      fontWeight: "inherit",
      fontSize: "inherit",
      lineHeight: "inherit",
      letterSpacing: "inherit"
    }
  };
  return Wt({
    htmlFontSize: l,
    pxToRem: h,
    fontFamily: a,
    fontSize: r,
    fontWeightLight: n,
    fontWeightRegular: o,
    fontWeightMedium: i,
    fontWeightBold: s,
    ...m
  }, d, {
    clone: !1
    // No need to clone deep
  });
}
const zx = 0.2, Wx = 0.14, Vx = 0.12;
function Ct(...t) {
  return [`${t[0]}px ${t[1]}px ${t[2]}px ${t[3]}px rgba(0,0,0,${zx})`, `${t[4]}px ${t[5]}px ${t[6]}px ${t[7]}px rgba(0,0,0,${Wx})`, `${t[8]}px ${t[9]}px ${t[10]}px ${t[11]}px rgba(0,0,0,${Vx})`].join(",");
}
const Hx = ["none", Ct(0, 2, 1, -1, 0, 1, 1, 0, 0, 1, 3, 0), Ct(0, 3, 1, -2, 0, 2, 2, 0, 0, 1, 5, 0), Ct(0, 3, 3, -2, 0, 3, 4, 0, 0, 1, 8, 0), Ct(0, 2, 4, -1, 0, 4, 5, 0, 0, 1, 10, 0), Ct(0, 3, 5, -1, 0, 5, 8, 0, 0, 1, 14, 0), Ct(0, 3, 5, -1, 0, 6, 10, 0, 0, 1, 18, 0), Ct(0, 4, 5, -2, 0, 7, 10, 1, 0, 2, 16, 1), Ct(0, 5, 5, -3, 0, 8, 10, 1, 0, 3, 14, 2), Ct(0, 5, 6, -3, 0, 9, 12, 1, 0, 3, 16, 2), Ct(0, 6, 6, -3, 0, 10, 14, 1, 0, 4, 18, 3), Ct(0, 6, 7, -4, 0, 11, 15, 1, 0, 4, 20, 3), Ct(0, 7, 8, -4, 0, 12, 17, 2, 0, 5, 22, 4), Ct(0, 7, 8, -4, 0, 13, 19, 2, 0, 5, 24, 4), Ct(0, 7, 9, -4, 0, 14, 21, 2, 0, 5, 26, 4), Ct(0, 8, 9, -5, 0, 15, 22, 2, 0, 6, 28, 5), Ct(0, 8, 10, -5, 0, 16, 24, 2, 0, 6, 30, 5), Ct(0, 8, 11, -5, 0, 17, 26, 2, 0, 6, 32, 5), Ct(0, 9, 11, -5, 0, 18, 28, 2, 0, 7, 34, 6), Ct(0, 9, 12, -6, 0, 19, 29, 2, 0, 7, 36, 6), Ct(0, 10, 13, -6, 0, 20, 31, 3, 0, 8, 38, 7), Ct(0, 10, 13, -6, 0, 21, 33, 3, 0, 8, 40, 7), Ct(0, 10, 14, -6, 0, 22, 35, 3, 0, 8, 42, 7), Ct(0, 11, 14, -7, 0, 23, 36, 3, 0, 9, 44, 8), Ct(0, 11, 15, -7, 0, 24, 38, 3, 0, 9, 46, 8)], Kx = {
  // This is the most common easing curve.
  easeInOut: "cubic-bezier(0.4, 0, 0.2, 1)",
  // Objects enter the screen at full velocity from off-screen and
  // slowly decelerate to a resting point.
  easeOut: "cubic-bezier(0.0, 0, 0.2, 1)",
  // Objects leave the screen at full velocity. They do not decelerate when off-screen.
  easeIn: "cubic-bezier(0.4, 0, 1, 1)",
  // The sharp curve is used by objects that may return to the screen at any time.
  sharp: "cubic-bezier(0.4, 0, 0.6, 1)"
}, Ux = {
  shortest: 150,
  shorter: 200,
  short: 250,
  // most basic recommended timing
  standard: 300,
  // this is to be used in complex animations
  complex: 375,
  // recommended when something is entering screen
  enteringScreen: 225,
  // recommended when something is leaving screen
  leavingScreen: 195
};
function Cc(t) {
  return `${Math.round(t)}ms`;
}
function qx(t) {
  if (!t)
    return 0;
  const e = t / 36;
  return Math.min(Math.round((4 + 15 * e ** 0.25 + e / 5) * 10), 3e3);
}
function Gx(t) {
  const e = {
    ...Kx,
    ...t.easing
  }, a = {
    ...Ux,
    ...t.duration
  };
  return {
    getAutoHeightDuration: qx,
    create: (n = ["all"], o = {}) => {
      const {
        duration: i = a.standard,
        easing: s = e.easeInOut,
        delay: l = 0,
        ...u
      } = o;
      return (Array.isArray(n) ? n : [n]).map((c) => `${c} ${typeof i == "string" ? i : Cc(i)} ${s} ${typeof l == "string" ? l : Cc(l)}`).join(",");
    },
    ...t,
    easing: e,
    duration: a
  };
}
const Yx = {
  mobileStepper: 1e3,
  fab: 1050,
  speedDial: 1050,
  appBar: 1100,
  drawer: 1200,
  modal: 1300,
  snackbar: 1400,
  tooltip: 1500
};
function Xx(t) {
  return lr(t) || typeof t > "u" || typeof t == "string" || typeof t == "boolean" || typeof t == "number" || Array.isArray(t);
}
function md(t = {}) {
  const e = {
    ...t
  };
  function a(r) {
    const n = Object.entries(r);
    for (let o = 0; o < n.length; o++) {
      const [i, s] = n[o];
      !Xx(s) || i.startsWith("unstable_") ? delete r[i] : lr(s) && (r[i] = {
        ...s
      }, a(r[i]));
    }
  }
  return a(e), `import { unstable_createBreakpoints as createBreakpoints, createTransitions } from '@mui/material/styles';

const theme = ${JSON.stringify(e, null, 2)};

theme.breakpoints = createBreakpoints(theme.breakpoints || {});
theme.transitions = createTransitions(theme.transitions || {});

export default theme;`;
}
function Sc(t) {
  return typeof t == "number" ? `${(t * 100).toFixed(0)}%` : `calc((${t}) * 100%)`;
}
const Jx = (t) => {
  if (!Number.isNaN(+t))
    return +t;
  const e = t.match(/\d*\.?\d+/g);
  if (!e)
    return 0;
  let a = 0;
  for (let r = 0; r < e.length; r += 1)
    a += +e[r];
  return a;
};
function Zx(t) {
  Object.assign(t, {
    alpha(e, a) {
      const r = this || t;
      return r.colorSpace ? `oklch(from ${e} l c h / ${typeof a == "string" ? `calc(${a})` : a})` : r.vars ? `rgba(${e.replace(/var\(--([^,\s)]+)(?:,[^)]+)?\)+/g, "var(--$1Channel)")} / ${typeof a == "string" ? `calc(${a})` : a})` : dd(e, Jx(a));
    },
    lighten(e, a) {
      const r = this || t;
      return r.colorSpace ? `color-mix(in ${r.colorSpace}, ${e}, #fff ${Sc(a)})` : yo(e, a);
    },
    darken(e, a) {
      const r = this || t;
      return r.colorSpace ? `color-mix(in ${r.colorSpace}, ${e}, #000 ${Sc(a)})` : bo(e, a);
    }
  });
}
function Wi(t = {}, ...e) {
  const {
    breakpoints: a,
    mixins: r = {},
    spacing: n,
    palette: o = {},
    transitions: i = {},
    typography: s = {},
    shape: l,
    colorSpace: u,
    ...c
  } = t;
  if (t.vars && // The error should throw only for the root theme creation because user is not allowed to use a custom node `vars`.
  // `generateThemeVars` is the closest identifier for checking that the `options` is a result of `createTheme` with CSS variables so that user can create new theme for nested ThemeProvider.
  t.generateThemeVars === void 0)
    throw new Error(Kr(20));
  const d = ks({
    ...o,
    colorSpace: u
  }), p = od(t);
  let h = Wt(p, {
    mixins: Dx(p.breakpoints, r),
    palette: d,
    // Don't use [...shadows] until you've verified its transpiled code is not invoking the iterator protocol.
    shadows: Hx.slice(),
    typography: Fx(d, s),
    transitions: Gx(i),
    zIndex: {
      ...Yx
    }
  });
  return h = Wt(h, c), h = e.reduce((g, m) => Wt(g, m), h), h.unstable_sxConfig = {
    ...po,
    ...c == null ? void 0 : c.unstable_sxConfig
  }, h.unstable_sx = function(m) {
    return Sn({
      sx: m,
      theme: this
    });
  }, h.toRuntimeSource = md, Zx(h), h;
}
function Qx(t) {
  let e;
  return t < 1 ? e = 5.11916 * t ** 2 : e = 4.5 * Math.log(t + 1) + 2, Math.round(e * 10) / 1e3;
}
const e2 = [...Array(25)].map((t, e) => {
  if (e === 0)
    return "none";
  const a = Qx(e);
  return `linear-gradient(rgba(255 255 255 / ${a}), rgba(255 255 255 / ${a}))`;
});
function gd(t) {
  return {
    inputPlaceholder: t === "dark" ? 0.5 : 0.42,
    inputUnderline: t === "dark" ? 0.7 : 0.42,
    switchTrackDisabled: t === "dark" ? 0.2 : 0.12,
    switchTrack: t === "dark" ? 0.3 : 0.38
  };
}
function vd(t) {
  return t === "dark" ? e2 : [];
}
function t2(t) {
  const {
    palette: e = {
      mode: "light"
    },
    // need to cast to avoid module augmentation test
    opacity: a,
    overlays: r,
    colorSpace: n,
    ...o
  } = t, i = ks({
    ...e,
    colorSpace: n
  });
  return {
    palette: i,
    opacity: {
      ...gd(i.mode),
      ...a
    },
    overlays: r || vd(i.mode),
    ...o
  };
}
function r2(t) {
  var e;
  return !!t[0].match(/(cssVarPrefix|colorSchemeSelector|modularCssLayers|rootSelector|typography|mixins|breakpoints|direction|transitions)/) || !!t[0].match(/sxConfig$/) || // ends with sxConfig
  t[0] === "palette" && !!((e = t[1]) != null && e.match(/(mode|contrastThreshold|tonalOffset)/));
}
const n2 = (t) => [...[...Array(25)].map((e, a) => `--${t ? `${t}-` : ""}overlays-${a}`), `--${t ? `${t}-` : ""}palette-AppBar-darkBg`, `--${t ? `${t}-` : ""}palette-AppBar-darkColor`], a2 = (t) => (e, a) => {
  const r = t.rootSelector || ":root", n = t.colorSchemeSelector;
  let o = n;
  if (n === "class" && (o = ".%s"), n === "data" && (o = "[data-%s]"), n != null && n.startsWith("data-") && !n.includes("%s") && (o = `[${n}="%s"]`), t.defaultColorScheme === e) {
    if (e === "dark") {
      const i = {};
      return n2(t.cssVarPrefix).forEach((s) => {
        i[s] = a[s], delete a[s];
      }), o === "media" ? {
        [r]: a,
        "@media (prefers-color-scheme: dark)": {
          [r]: i
        }
      } : o ? {
        [o.replace("%s", e)]: i,
        [`${r}, ${o.replace("%s", e)}`]: a
      } : {
        [r]: {
          ...a,
          ...i
        }
      };
    }
    if (o && o !== "media")
      return `${r}, ${o.replace("%s", String(e))}`;
  } else if (e) {
    if (o === "media")
      return {
        [`@media (prefers-color-scheme: ${String(e)})`]: {
          [r]: a
        }
      };
    if (o)
      return o.replace("%s", String(e));
  }
  return r;
};
function o2(t, e) {
  e.forEach((a) => {
    t[a] || (t[a] = {});
  });
}
function ne(t, e, a) {
  !t[e] && a && (t[e] = a);
}
function Wn(t) {
  return typeof t != "string" || !t.startsWith("hsl") ? t : ud(t);
}
function sr(t, e) {
  `${e}Channel` in t || (t[`${e}Channel`] = zn(Wn(t[e])));
}
function i2(t) {
  return typeof t == "number" ? `${t}px` : typeof t == "string" || typeof t == "function" || Array.isArray(t) ? t : "8px";
}
const Qt = (t) => {
  try {
    return t();
  } catch {
  }
}, s2 = (t = "mui") => Ex(t);
function di(t, e, a, r, n) {
  if (!a)
    return;
  a = a === !0 ? {} : a;
  const o = n === "dark" ? "dark" : "light";
  if (!r) {
    e[n] = t2({
      ...a,
      palette: {
        mode: o,
        ...a == null ? void 0 : a.palette
      },
      colorSpace: t
    });
    return;
  }
  const {
    palette: i,
    ...s
  } = Wi({
    ...r,
    palette: {
      mode: o,
      ...a == null ? void 0 : a.palette
    },
    colorSpace: t
  });
  return e[n] = {
    ...a,
    palette: i,
    opacity: {
      ...gd(o),
      ...a == null ? void 0 : a.opacity
    },
    overlays: (a == null ? void 0 : a.overlays) || vd(o)
  }, s;
}
function l2(t = {}, ...e) {
  const {
    colorSchemes: a = {
      light: !0
    },
    defaultColorScheme: r,
    disableCssColorScheme: n = !1,
    cssVarPrefix: o = "mui",
    nativeColor: i = !1,
    shouldSkipGeneratingVar: s = r2,
    colorSchemeSelector: l = a.light && a.dark ? "media" : void 0,
    rootSelector: u = ":root",
    ...c
  } = t, d = Object.keys(a)[0], p = r || (a.light && d !== "light" ? "light" : d), h = s2(o), {
    [p]: g,
    light: m,
    dark: y,
    ...w
  } = a, k = {
    ...w
  };
  let j = g;
  if ((p === "dark" && !("dark" in a) || p === "light" && !("light" in a)) && (j = !0), !j)
    throw new Error(Kr(21, p));
  let f;
  i && (f = "oklch");
  const x = di(f, k, j, c, p);
  m && !k.light && di(f, k, m, void 0, "light"), y && !k.dark && di(f, k, y, void 0, "dark");
  let E = {
    defaultColorScheme: p,
    ...x,
    cssVarPrefix: o,
    colorSchemeSelector: l,
    rootSelector: u,
    getCssVar: h,
    colorSchemes: k,
    font: {
      ...Lx(x.typography),
      ...x.font
    },
    spacing: i2(c.spacing)
  };
  Object.keys(E.colorSchemes).forEach((A) => {
    const v = E.colorSchemes[A].palette, C = (D) => {
      const $ = D.split("-"), H = $[1], F = $[2];
      return h(D, v[H][F]);
    };
    v.mode === "light" && (ne(v.common, "background", "#fff"), ne(v.common, "onBackground", "#000")), v.mode === "dark" && (ne(v.common, "background", "#000"), ne(v.common, "onBackground", "#fff"));
    function M(D, $, H) {
      if (f) {
        let F;
        return D === Ar && (F = `transparent ${((1 - H) * 100).toFixed(0)}%`), D === dt && (F = `#000 ${(H * 100).toFixed(0)}%`), D === ft && (F = `#fff ${(H * 100).toFixed(0)}%`), `color-mix(in ${f}, ${$}, ${F})`;
      }
      return D($, H);
    }
    if (o2(v, ["Alert", "AppBar", "Avatar", "Button", "Chip", "FilledInput", "LinearProgress", "Skeleton", "Slider", "SnackbarContent", "SpeedDialAction", "StepConnector", "StepContent", "Switch", "TableCell", "Tooltip"]), v.mode === "light") {
      ne(v.Alert, "errorColor", M(dt, v.error.light, 0.6)), ne(v.Alert, "infoColor", M(dt, v.info.light, 0.6)), ne(v.Alert, "successColor", M(dt, v.success.light, 0.6)), ne(v.Alert, "warningColor", M(dt, v.warning.light, 0.6)), ne(v.Alert, "errorFilledBg", C("palette-error-main")), ne(v.Alert, "infoFilledBg", C("palette-info-main")), ne(v.Alert, "successFilledBg", C("palette-success-main")), ne(v.Alert, "warningFilledBg", C("palette-warning-main")), ne(v.Alert, "errorFilledColor", Qt(() => v.getContrastText(v.error.main))), ne(v.Alert, "infoFilledColor", Qt(() => v.getContrastText(v.info.main))), ne(v.Alert, "successFilledColor", Qt(() => v.getContrastText(v.success.main))), ne(v.Alert, "warningFilledColor", Qt(() => v.getContrastText(v.warning.main))), ne(v.Alert, "errorStandardBg", M(ft, v.error.light, 0.9)), ne(v.Alert, "infoStandardBg", M(ft, v.info.light, 0.9)), ne(v.Alert, "successStandardBg", M(ft, v.success.light, 0.9)), ne(v.Alert, "warningStandardBg", M(ft, v.warning.light, 0.9)), ne(v.Alert, "errorIconColor", C("palette-error-main")), ne(v.Alert, "infoIconColor", C("palette-info-main")), ne(v.Alert, "successIconColor", C("palette-success-main")), ne(v.Alert, "warningIconColor", C("palette-warning-main")), ne(v.AppBar, "defaultBg", C("palette-grey-100")), ne(v.Avatar, "defaultBg", C("palette-grey-400")), ne(v.Button, "inheritContainedBg", C("palette-grey-300")), ne(v.Button, "inheritContainedHoverBg", C("palette-grey-A100")), ne(v.Chip, "defaultBorder", C("palette-grey-400")), ne(v.Chip, "defaultAvatarColor", C("palette-grey-700")), ne(v.Chip, "defaultIconColor", C("palette-grey-700")), ne(v.FilledInput, "bg", "rgba(0, 0, 0, 0.06)"), ne(v.FilledInput, "hoverBg", "rgba(0, 0, 0, 0.09)"), ne(v.FilledInput, "disabledBg", "rgba(0, 0, 0, 0.12)"), ne(v.LinearProgress, "primaryBg", M(ft, v.primary.main, 0.62)), ne(v.LinearProgress, "secondaryBg", M(ft, v.secondary.main, 0.62)), ne(v.LinearProgress, "errorBg", M(ft, v.error.main, 0.62)), ne(v.LinearProgress, "infoBg", M(ft, v.info.main, 0.62)), ne(v.LinearProgress, "successBg", M(ft, v.success.main, 0.62)), ne(v.LinearProgress, "warningBg", M(ft, v.warning.main, 0.62)), ne(v.Skeleton, "bg", f ? M(Ar, v.text.primary, 0.11) : `rgba(${C("palette-text-primaryChannel")} / 0.11)`), ne(v.Slider, "primaryTrack", M(ft, v.primary.main, 0.62)), ne(v.Slider, "secondaryTrack", M(ft, v.secondary.main, 0.62)), ne(v.Slider, "errorTrack", M(ft, v.error.main, 0.62)), ne(v.Slider, "infoTrack", M(ft, v.info.main, 0.62)), ne(v.Slider, "successTrack", M(ft, v.success.main, 0.62)), ne(v.Slider, "warningTrack", M(ft, v.warning.main, 0.62));
      const D = f ? M(dt, v.background.default, 0.6825) : wa(v.background.default, 0.8);
      ne(v.SnackbarContent, "bg", D), ne(v.SnackbarContent, "color", Qt(() => f ? zi.text.primary : v.getContrastText(D))), ne(v.SpeedDialAction, "fabHoverBg", wa(v.background.paper, 0.15)), ne(v.StepConnector, "border", C("palette-grey-400")), ne(v.StepContent, "border", C("palette-grey-400")), ne(v.Switch, "defaultColor", C("palette-common-white")), ne(v.Switch, "defaultDisabledColor", C("palette-grey-100")), ne(v.Switch, "primaryDisabledColor", M(ft, v.primary.main, 0.62)), ne(v.Switch, "secondaryDisabledColor", M(ft, v.secondary.main, 0.62)), ne(v.Switch, "errorDisabledColor", M(ft, v.error.main, 0.62)), ne(v.Switch, "infoDisabledColor", M(ft, v.info.main, 0.62)), ne(v.Switch, "successDisabledColor", M(ft, v.success.main, 0.62)), ne(v.Switch, "warningDisabledColor", M(ft, v.warning.main, 0.62)), ne(v.TableCell, "border", M(ft, M(Ar, v.divider, 1), 0.88)), ne(v.Tooltip, "bg", M(Ar, v.grey[700], 0.92));
    }
    if (v.mode === "dark") {
      ne(v.Alert, "errorColor", M(ft, v.error.light, 0.6)), ne(v.Alert, "infoColor", M(ft, v.info.light, 0.6)), ne(v.Alert, "successColor", M(ft, v.success.light, 0.6)), ne(v.Alert, "warningColor", M(ft, v.warning.light, 0.6)), ne(v.Alert, "errorFilledBg", C("palette-error-dark")), ne(v.Alert, "infoFilledBg", C("palette-info-dark")), ne(v.Alert, "successFilledBg", C("palette-success-dark")), ne(v.Alert, "warningFilledBg", C("palette-warning-dark")), ne(v.Alert, "errorFilledColor", Qt(() => v.getContrastText(v.error.dark))), ne(v.Alert, "infoFilledColor", Qt(() => v.getContrastText(v.info.dark))), ne(v.Alert, "successFilledColor", Qt(() => v.getContrastText(v.success.dark))), ne(v.Alert, "warningFilledColor", Qt(() => v.getContrastText(v.warning.dark))), ne(v.Alert, "errorStandardBg", M(dt, v.error.light, 0.9)), ne(v.Alert, "infoStandardBg", M(dt, v.info.light, 0.9)), ne(v.Alert, "successStandardBg", M(dt, v.success.light, 0.9)), ne(v.Alert, "warningStandardBg", M(dt, v.warning.light, 0.9)), ne(v.Alert, "errorIconColor", C("palette-error-main")), ne(v.Alert, "infoIconColor", C("palette-info-main")), ne(v.Alert, "successIconColor", C("palette-success-main")), ne(v.Alert, "warningIconColor", C("palette-warning-main")), ne(v.AppBar, "defaultBg", C("palette-grey-900")), ne(v.AppBar, "darkBg", C("palette-background-paper")), ne(v.AppBar, "darkColor", C("palette-text-primary")), ne(v.Avatar, "defaultBg", C("palette-grey-600")), ne(v.Button, "inheritContainedBg", C("palette-grey-800")), ne(v.Button, "inheritContainedHoverBg", C("palette-grey-700")), ne(v.Chip, "defaultBorder", C("palette-grey-700")), ne(v.Chip, "defaultAvatarColor", C("palette-grey-300")), ne(v.Chip, "defaultIconColor", C("palette-grey-300")), ne(v.FilledInput, "bg", "rgba(255, 255, 255, 0.09)"), ne(v.FilledInput, "hoverBg", "rgba(255, 255, 255, 0.13)"), ne(v.FilledInput, "disabledBg", "rgba(255, 255, 255, 0.12)"), ne(v.LinearProgress, "primaryBg", M(dt, v.primary.main, 0.5)), ne(v.LinearProgress, "secondaryBg", M(dt, v.secondary.main, 0.5)), ne(v.LinearProgress, "errorBg", M(dt, v.error.main, 0.5)), ne(v.LinearProgress, "infoBg", M(dt, v.info.main, 0.5)), ne(v.LinearProgress, "successBg", M(dt, v.success.main, 0.5)), ne(v.LinearProgress, "warningBg", M(dt, v.warning.main, 0.5)), ne(v.Skeleton, "bg", f ? M(Ar, v.text.primary, 0.13) : `rgba(${C("palette-text-primaryChannel")} / 0.13)`), ne(v.Slider, "primaryTrack", M(dt, v.primary.main, 0.5)), ne(v.Slider, "secondaryTrack", M(dt, v.secondary.main, 0.5)), ne(v.Slider, "errorTrack", M(dt, v.error.main, 0.5)), ne(v.Slider, "infoTrack", M(dt, v.info.main, 0.5)), ne(v.Slider, "successTrack", M(dt, v.success.main, 0.5)), ne(v.Slider, "warningTrack", M(dt, v.warning.main, 0.5));
      const D = f ? M(ft, v.background.default, 0.985) : wa(v.background.default, 0.98);
      ne(v.SnackbarContent, "bg", D), ne(v.SnackbarContent, "color", Qt(() => f ? pd.text.primary : v.getContrastText(D))), ne(v.SpeedDialAction, "fabHoverBg", wa(v.background.paper, 0.15)), ne(v.StepConnector, "border", C("palette-grey-600")), ne(v.StepContent, "border", C("palette-grey-600")), ne(v.Switch, "defaultColor", C("palette-grey-300")), ne(v.Switch, "defaultDisabledColor", C("palette-grey-600")), ne(v.Switch, "primaryDisabledColor", M(dt, v.primary.main, 0.55)), ne(v.Switch, "secondaryDisabledColor", M(dt, v.secondary.main, 0.55)), ne(v.Switch, "errorDisabledColor", M(dt, v.error.main, 0.55)), ne(v.Switch, "infoDisabledColor", M(dt, v.info.main, 0.55)), ne(v.Switch, "successDisabledColor", M(dt, v.success.main, 0.55)), ne(v.Switch, "warningDisabledColor", M(dt, v.warning.main, 0.55)), ne(v.TableCell, "border", M(dt, M(Ar, v.divider, 1), 0.68)), ne(v.Tooltip, "bg", M(Ar, v.grey[700], 0.92));
    }
    sr(v.background, "default"), sr(v.background, "paper"), sr(v.common, "background"), sr(v.common, "onBackground"), sr(v, "divider"), Object.keys(v).forEach((D) => {
      const $ = v[D];
      D !== "tonalOffset" && $ && typeof $ == "object" && ($.main && ne(v[D], "mainChannel", zn(Wn($.main))), $.light && ne(v[D], "lightChannel", zn(Wn($.light))), $.dark && ne(v[D], "darkChannel", zn(Wn($.dark))), $.contrastText && ne(v[D], "contrastTextChannel", zn(Wn($.contrastText))), D === "text" && (sr(v[D], "primary"), sr(v[D], "secondary")), D === "action" && ($.active && sr(v[D], "active"), $.selected && sr(v[D], "selected")));
    });
  }), E = e.reduce((A, v) => Wt(A, v), E);
  const L = {
    prefix: o,
    disableCssColorScheme: n,
    shouldSkipGeneratingVar: s,
    getSelector: a2(E),
    enableContrastVars: i
  }, {
    vars: W,
    generateThemeVars: O,
    generateStyleSheets: P
  } = Rx(E, L);
  return E.vars = W, Object.entries(E.colorSchemes[E.defaultColorScheme]).forEach(([A, v]) => {
    E[A] = v;
  }), E.generateThemeVars = O, E.generateStyleSheets = P, E.generateSpacing = function() {
    return ad(c.spacing, ys(this));
  }, E.getColorSchemeSelector = Tx(l), E.spacing = E.generateSpacing(), E.shouldSkipGeneratingVar = s, E.unstable_sxConfig = {
    ...po,
    ...c == null ? void 0 : c.unstable_sxConfig
  }, E.unstable_sx = function(v) {
    return Sn({
      sx: v,
      theme: this
    });
  }, E.toRuntimeSource = md, E;
}
function wc(t, e, a) {
  t.colorSchemes && a && (t.colorSchemes[e] = {
    ...a !== !0 && a,
    palette: ks({
      ...a === !0 ? {} : a.palette,
      mode: e
    })
    // cast type to skip module augmentation test
  });
}
function c2(t = {}, ...e) {
  const {
    palette: a,
    cssVariables: r = !1,
    colorSchemes: n = a ? void 0 : {
      light: !0
    },
    defaultColorScheme: o = a == null ? void 0 : a.mode,
    ...i
  } = t, s = o || "light", l = n == null ? void 0 : n[s], u = {
    ...n,
    ...a ? {
      [s]: {
        ...typeof l != "boolean" && l,
        palette: a
      }
    } : void 0
  };
  if (r === !1) {
    if (!("colorSchemes" in t))
      return Wi(t, ...e);
    let c = a;
    "palette" in t || u[s] && (u[s] !== !0 ? c = u[s].palette : s === "dark" && (c = {
      mode: "dark"
    }));
    const d = Wi({
      ...t,
      palette: c
    }, ...e);
    return d.defaultColorScheme = s, d.colorSchemes = u, d.palette.mode === "light" && (d.colorSchemes.light = {
      ...u.light !== !0 && u.light,
      palette: d.palette
    }, wc(d, "dark", u.dark)), d.palette.mode === "dark" && (d.colorSchemes.dark = {
      ...u.dark !== !0 && u.dark,
      palette: d.palette
    }, wc(d, "light", u.light)), d;
  }
  return !a && !("light" in u) && s === "light" && (u.light = !0), l2({
    ...i,
    colorSchemes: u,
    defaultColorScheme: s,
    ...typeof r != "boolean" && r
  }, ...e);
}
const u2 = c2(), d2 = "$$material";
function f2(t) {
  return t !== "ownerState" && t !== "theme" && t !== "sx" && t !== "as";
}
const p2 = (t) => f2(t) && t !== "classes", h2 = cx({
  themeId: d2,
  defaultTheme: u2,
  rootShouldForwardProp: p2
}), m2 = wx;
function g2(t) {
  return Sx(t);
}
function v2(t) {
  return id("MuiSvgIcon", t);
}
ox("MuiSvgIcon", ["root", "colorPrimary", "colorSecondary", "colorAction", "colorError", "colorDisabled", "fontSizeInherit", "fontSizeSmall", "fontSizeMedium", "fontSizeLarge"]);
const b2 = (t) => {
  const {
    color: e,
    fontSize: a,
    classes: r
  } = t, n = {
    root: ["root", e !== "inherit" && `color${Cn(e)}`, `fontSize${Cn(a)}`]
  };
  return uy(n, v2, r);
}, y2 = h2("svg", {
  name: "MuiSvgIcon",
  slot: "Root",
  overridesResolver: (t, e) => {
    const {
      ownerState: a
    } = t;
    return [e.root, a.color !== "inherit" && e[`color${Cn(a.color)}`], e[`fontSize${Cn(a.fontSize)}`]];
  }
})(m2(({
  theme: t
}) => {
  var e, a, r, n, o, i, s, l, u, c, d, p, h, g;
  return {
    userSelect: "none",
    width: "1em",
    height: "1em",
    display: "inline-block",
    flexShrink: 0,
    transition: (n = (e = t.transitions) == null ? void 0 : e.create) == null ? void 0 : n.call(e, "fill", {
      duration: (r = (a = (t.vars ?? t).transitions) == null ? void 0 : a.duration) == null ? void 0 : r.shorter
    }),
    variants: [
      {
        props: (m) => !m.hasSvgAsChild,
        style: {
          // the <svg> will define the property that has `currentColor`
          // for example heroicons uses fill="none" and stroke="currentColor"
          fill: "currentColor"
        }
      },
      {
        props: {
          fontSize: "inherit"
        },
        style: {
          fontSize: "inherit"
        }
      },
      {
        props: {
          fontSize: "small"
        },
        style: {
          fontSize: ((i = (o = t.typography) == null ? void 0 : o.pxToRem) == null ? void 0 : i.call(o, 20)) || "1.25rem"
        }
      },
      {
        props: {
          fontSize: "medium"
        },
        style: {
          fontSize: ((l = (s = t.typography) == null ? void 0 : s.pxToRem) == null ? void 0 : l.call(s, 24)) || "1.5rem"
        }
      },
      {
        props: {
          fontSize: "large"
        },
        style: {
          fontSize: ((c = (u = t.typography) == null ? void 0 : u.pxToRem) == null ? void 0 : c.call(u, 35)) || "2.1875rem"
        }
      },
      // TODO v5 deprecate color prop, v6 remove for sx
      ...Object.entries((t.vars ?? t).palette).filter(([, m]) => m && m.main).map(([m]) => {
        var y, w;
        return {
          props: {
            color: m
          },
          style: {
            color: (w = (y = (t.vars ?? t).palette) == null ? void 0 : y[m]) == null ? void 0 : w.main
          }
        };
      }),
      {
        props: {
          color: "action"
        },
        style: {
          color: (p = (d = (t.vars ?? t).palette) == null ? void 0 : d.action) == null ? void 0 : p.active
        }
      },
      {
        props: {
          color: "disabled"
        },
        style: {
          color: (g = (h = (t.vars ?? t).palette) == null ? void 0 : h.action) == null ? void 0 : g.disabled
        }
      },
      {
        props: {
          color: "inherit"
        },
        style: {
          color: void 0
        }
      }
    ]
  };
})), Vi = /* @__PURE__ */ b.forwardRef(function(e, a) {
  const r = g2({
    props: e,
    name: "MuiSvgIcon"
  }), {
    children: n,
    className: o,
    color: i = "inherit",
    component: s = "svg",
    fontSize: l = "medium",
    htmlColor: u,
    inheritViewBox: c = !1,
    titleAccess: d,
    viewBox: p = "0 0 24 24",
    ...h
  } = r, g = /* @__PURE__ */ b.isValidElement(n) && n.type === "svg", m = {
    ...r,
    color: i,
    component: s,
    fontSize: l,
    instanceFontSize: e.fontSize,
    inheritViewBox: c,
    viewBox: p,
    hasSvgAsChild: g
  }, y = {};
  c || (y.viewBox = p);
  const w = b2(m);
  return /* @__PURE__ */ B.jsxs(y2, {
    as: s,
    className: cy(w.root, o),
    focusable: "false",
    color: u,
    "aria-hidden": d ? void 0 : !0,
    role: d ? "img" : void 0,
    ref: a,
    ...y,
    ...h,
    ...g && n.props,
    ownerState: m,
    children: [g ? n.props.children : n, d ? /* @__PURE__ */ B.jsx("title", {
      children: d
    }) : null]
  });
});
Vi.muiName = "SvgIcon";
function x2(t, e) {
  function a(r, n) {
    return /* @__PURE__ */ B.jsx(Vi, {
      "data-testid": void 0,
      ref: n,
      ...r,
      children: t
    });
  }
  return a.muiName = Vi.muiName, /* @__PURE__ */ b.memo(/* @__PURE__ */ b.forwardRef(a));
}
const C2 = x2(/* @__PURE__ */ B.jsx("path", {
  d: "M16 1H4c-1.1 0-2 .9-2 2v14h2V3h12zm3 4H8c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h11c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2m0 16H8V7h11z"
})), S2 = (t) => {
  const { value: e, testId: a, size: r, stopPropagation: n } = t, o = e, i = "Copy to Clipboard", s = "Copied", [l, u] = jt(i), c = () => {
    u(i);
  }, d = (p) => {
    u(s), Fu(o), n && p.stopPropagation();
  };
  return /* @__PURE__ */ B.jsx(
    ms,
    {
      title: l,
      onClose: c,
      placement: "top-end",
      children: /* @__PURE__ */ B.jsx(ve, { component: "span", children: /* @__PURE__ */ B.jsx(su, { size: "small", color: "primary", onClick: (p) => d(p), children: /* @__PURE__ */ B.jsx(C2, {}) }) })
    }
  );
}, w2 = ar(
  (t) => fr({
    infoTooltipDark: {
      color: t.palette.grey[100],
      backgroundColor: t.palette.secondary.dark,
      borderRadius: 5
    },
    infoArrowDark: {
      color: t.palette.secondary.dark
    },
    infoTooltipLight: {
      color: t.palette.secondary.dark,
      backgroundColor: t.palette.common.white,
      borderRadius: 5,
      maxWidth: t.spacing(53)
    },
    infoArrowLight: {
      color: t.palette.common.white
    }
  })
), E2 = ar(
  (t) => fr({
    divider: {
      marginTop: t.spacing(1),
      marginBottom: t.spacing(1),
      backgroundColor: t.palette.grey[100]
    },
    buttonLink: {
      color: t.palette.primary.main,
      cursor: "pointer",
      marginTop: t.spacing(1.5),
      textDecoration: "none"
    },
    dividerDark: {
      backgroundColor: t.palette.grey[100]
    },
    exampleContent: {
      fontWeight: 100,
      marginTop: t.spacing(1),
      marginBottom: t.spacing(1)
    },
    exampleContentDark: {
      color: t.palette.grey[100]
    },
    exampleContentLight: {
      color: t.palette.secondary.dark
    }
  })
);
function k2(t) {
  const { children: e, classes: a, disabled: r, title: n, ...o } = t;
  return r ? /* @__PURE__ */ B.jsx(B.Fragment, { children: e }) : /* @__PURE__ */ B.jsx(B.Fragment, { children: e && n && /* @__PURE__ */ B.jsx(
    ms,
    {
      ...o,
      classes: a,
      interactive: !0,
      title: n,
      children: e
    }
  ) });
}
function Ec(t) {
  const {
    children: e,
    heading: a,
    disabled: r,
    content: n,
    example: o,
    action: i,
    darken: s = !0,
    ...l
  } = t, u = E2(), c = w2(), d = /* @__PURE__ */ B.jsxs(ve, { p: 0.5, children: [
    a && /* @__PURE__ */ B.jsx(ve, { mb: n ? 1 : 0, children: /* @__PURE__ */ B.jsx(Mt, { variant: "h5", children: a }) }),
    n && /* @__PURE__ */ B.jsx(Mt, { variant: "body2", children: n }),
    (i || o) && /* @__PURE__ */ B.jsx(Zg, { className: u.divider }),
    o && /* @__PURE__ */ B.jsxs(
      Mt,
      {
        className: he({
          [u.exampleContent]: !0,
          [u.exampleContentDark]: s,
          [u.exampleContentLight]: !s
        }),
        variant: "body2",
        children: [
          "Eg: ",
          o
        ]
      }
    ),
    i && /* @__PURE__ */ B.jsx(
      k0,
      {
        href: i.link,
        className: u.buttonLink,
        target: "_blank",
        rel: "noreferrer",
        children: i.text
      }
    )
  ] });
  return e ? /* @__PURE__ */ B.jsx(
    k2,
    {
      title: d,
      disabled: r,
      classes: s ? {
        tooltip: c.infoTooltipDark,
        arrow: c.infoArrowDark
      } : {
        tooltip: c.infoTooltipLight,
        arrow: c.infoArrowLight
      },
      ...l,
      children: e
    }
  ) : null;
}
const bd = ar((t) => ({
  root: {
    padding: t.spacing(0.5, 1.5),
    width: "100%",
    minHeight: t.spacing(5),
    backgroundColor: t.palette.common.white,
    border: `1px solid ${t.palette.grey[100]}`,
    boxShadow: `0 1px 2px -1px ${Ye(
      t.palette.common.black,
      0.08
    )}, 0 -3px 9px 0 ${Ye(t.palette.common.black, 0.04)} inset`,
    borderRadius: 5,
    "&$multiline": {
      height: "auto",
      resize: "auto"
    },
    "&$multilineReadonly": {
      height: "auto",
      resize: "none",
      "& $textarea": {
        height: "auto",
        resize: "none"
      }
    },
    "&$multilineResizeIndicator": {
      height: "auto",
      resize: "none",
      "& $textarea": {
        height: "auto",
        resize: "none"
      }
    },
    "&$rounded": {
      paddingLeft: t.spacing(2)
    },
    "&:hover": {
      borderColor: "#ccd1f2"
    }
  },
  rootSmall: {
    minHeight: t.spacing(4)
  },
  rootLarge: {
    minHeight: t.spacing(7),
    borderRadius: 12,
    padding: t.spacing(1, 1, 1, 3)
  },
  textInput: (e) => ({
    minHeight: t.spacing(2.5),
    padding: t.spacing(0.125, 0),
    fontSize: t.typography[e.typography || "body1"].fontSize,
    fontWeight: t.typography[e.typography || "body1"].fontWeight,
    lineHeight: t.typography[e.typography || "body1"].lineHeight
  }),
  textInputLarge: (e) => ({
    fontSize: t.typography[e.typography || "overline"].fontSize,
    fontWeight: t.typography[e.typography || "overline"].fontWeight,
    lineHeight: t.typography[e.typography || "overline"].lineHeight
  }),
  textInputDisabled: {
    "&:hover": {
      borderColor: t.palette.grey[100]
    }
  },
  inputAdornedEnd: {
    "& .MuiInputAdornment-root": {
      marginRight: t.spacing(-1)
    }
  },
  inputAdornedEndAlignTop: {
    "& .MuiInputAdornment-root": {
      alignSelf: "flex-end",
      height: "auto",
      marginBottom: t.spacing(0.25)
    }
  },
  multiline: {},
  multilineReadonly: {},
  multilineResizeIndicator: {},
  rounded: {
    borderRadius: t.spacing(2.5)
  },
  focused: {
    borderColor: t.palette.primary.light,
    borderWidth: 1,
    boxShadow: `0 -3px 9px 0 ${Ye(
      t.palette.common.black,
      0.04
    )} inset, 0 0 0 2px #D8E1E8`,
    "&:hover": {
      borderColor: t.palette.primary.light
    }
  },
  error: {
    background: t.palette.error.light,
    borderColor: t.palette.error.main,
    boxShadow: `0 0 0 1px ${t.palette.error.light}, inset 0 2px 2px ${Ye(
      t.palette.error.light,
      0.07
    )}`,
    "&:hover": {
      borderColor: t.palette.error.main
    }
  },
  readOnlyDefault: {
    boxShadow: `0 0 0 1px ${Ye(
      t.palette.common.black,
      0.05
    )}, inset 0 2px 2px ${Ye(t.palette.common.black, 0.05)}`,
    border: "none",
    backgroundColor: "#E5E4E2"
  },
  readOnlyPlain: {
    boxShadow: "none",
    border: "none",
    backgroundColor: t.palette.common.white,
    paddingLeft: 0,
    paddingRight: 0
  },
  formLabel: {
    display: "flex",
    width: "100%",
    alignItems: "center",
    marginBottom: t.spacing(0.5)
  },
  formLabelAction: {
    marginLeft: "auto",
    display: "flex",
    alignItems: "center"
  },
  formLabelInfo: {
    marginLeft: t.spacing(1),
    display: "flex",
    alignItems: "center"
  },
  formOptional: {
    color: t.palette.grey[200],
    fontSize: t.spacing(1.4),
    marginLeft: t.spacing(1)
  },
  formLabelTooltip: {
    marginLeft: t.spacing(1),
    display: "flex",
    alignItems: "center"
  },
  inputGroup: {
    position: "relative"
  },
  tooltipIcon: {
    display: "flex",
    alignItems: "center",
    color: t.palette.secondary.main,
    cursor: "help",
    fontSize: t.spacing(1.75)
  },
  textarea: {
    resize: "both"
  },
  copyToClipboardInput: {
    backgroundColor: "transparent",
    border: "none",
    boxShadow: "none",
    paddingRight: t.spacing(5),
    minHeight: t.spacing(4.5),
    fontSize: t.typography.body2.fontSize,
    fontFamily: '"Roboto Mono", "Monaco", "Consolas", monospace',
    "&:hover": {
      borderColor: "transparent"
    }
  },
  textInputInfoIcon: {
    display: "flex",
    alignItems: "center",
    fontSize: t.spacing(1.75)
  }
})), $2 = (t, e) => {
  const a = bd(t), {
    label: r,
    width: n,
    readOnly: o,
    error: i,
    helperText: s,
    rows: l,
    multiline: u,
    optional: c,
    loading: d,
    tooltip: p,
    tooltipPlacement: h = "top",
    inputTooltip: g,
    size: m = "medium",
    info: y,
    actions: w,
    testId: k,
    variant: j = "default",
    rounded: f = !1,
    resizeIndicator: x = !0,
    ...E
  } = t, L = p && /* @__PURE__ */ B.jsx(
    Ec,
    {
      darken: !0,
      title: p,
      disabled: !p,
      placement: h,
      children: /* @__PURE__ */ B.jsx(ve, { className: a.tooltipIcon, children: /* @__PURE__ */ B.jsx(ve, { className: a.textInputInfoIcon, children: /* @__PURE__ */ B.jsx(Ub, { fontSize: "inherit" }) }) })
    }
  );
  return /* @__PURE__ */ B.jsxs(ve, { width: n, children: [
    (r || L || y || c || w) && /* @__PURE__ */ B.jsxs(ve, { className: a.formLabel, children: [
      r && /* @__PURE__ */ B.jsx(Mt, { component: "h6", variant: "body1", children: r }),
      y && /* @__PURE__ */ B.jsx(ve, { className: a.formLabelInfo, children: y }),
      L && /* @__PURE__ */ B.jsx(ve, { className: a.formLabelTooltip, children: L }),
      c && /* @__PURE__ */ B.jsx(Mt, { variant: "body2", className: a.formOptional, children: "(Optional)" }),
      w && /* @__PURE__ */ B.jsx(ve, { className: a.formLabelAction, children: w })
    ] }),
    /* @__PURE__ */ B.jsx(
      Ec,
      {
        darken: !0,
        heading: g,
        disabled: !g,
        placement: h,
        children: /* @__PURE__ */ B.jsx(ve, { className: a.inputGroup, children: /* @__PURE__ */ B.jsx(
          ro,
          {
            ref: e,
            classes: {
              root: he({
                [a.root]: !0,
                [a.rootSmall]: m === "small",
                [a.rootLarge]: m === "large",
                [a.readOnlyDefault]: o && j === "default",
                [a.readOnlyPlain]: o && j === "plain",
                [a.multiline]: u,
                [a.multilineReadonly]: u && o,
                [a.multilineResizeIndicator]: u && !x,
                [a.rounded]: f
              }),
              focused: a.focused,
              error: a.error,
              inputMultiline: a.textarea,
              input: he({
                [a.textInput]: !0,
                [a.textInputLarge]: m === "large"
              }),
              disabled: a.textInputDisabled,
              adornedEnd: he({
                [a.inputAdornedEnd]: !0,
                [a.inputAdornedEndAlignTop]: u
              })
            },
            readOnly: o,
            ...E,
            error: i,
            rows: l,
            multiline: u,
            "data-cyid": k
          }
        ) })
      }
    ),
    i && s && /* @__PURE__ */ B.jsx(El, { error: i, children: /* @__PURE__ */ B.jsxs(ve, { display: "flex", alignItems: "center", children: [
      /* @__PURE__ */ B.jsx(ve, { className: a.textInputInfoIcon, children: /* @__PURE__ */ B.jsx(Hb, { fontSize: "inherit" }) }),
      /* @__PURE__ */ B.jsx(ve, { ml: 1, children: s })
    ] }) }),
    d && s && /* @__PURE__ */ B.jsx(El, { children: /* @__PURE__ */ B.jsxs(ve, { display: "flex", alignItems: "center", children: [
      /* @__PURE__ */ B.jsx(Xn, { size: 12 }),
      /* @__PURE__ */ B.jsx(ve, { ml: 1, children: s })
    ] }) })
  ] });
}, yr = Rc($2), R2 = ar(
  (t) => fr({
    copyToClipboard: {
      display: "flex",
      width: "100%",
      position: "relative"
    },
    copyToClipboardInputWrap: {
      flexGrow: 1
    },
    copyToClipboardCard: {
      padding: t.spacing(2),
      borderRadius: t.spacing(1),
      flexGrow: 1
    },
    copyIconWrap: {
      display: "flex",
      alignItems: "center",
      position: "absolute",
      bottom: t.spacing(0.625),
      right: t.spacing(0.5)
    },
    copyIconWrapCard: {
      top: t.spacing(1.25),
      right: t.spacing(1)
    }
  })
), Ea = (t) => {
  const e = R2(), a = bd(t), {
    size: r,
    value: n,
    component: o = "textInput",
    endAdornment: i,
    testId: s,
    isMultiline: l,
    ...u
  } = t, c = n;
  return /* @__PURE__ */ B.jsxs(
    ve,
    {
      className: e.copyToClipboard,
      "data-cyid": `${s}-copy-to-clipboard`,
      children: [
        o === "textInput" && /* @__PURE__ */ B.jsx(ve, { className: e.copyToClipboardInputWrap, children: /* @__PURE__ */ B.jsx(
          yr,
          {
            value: n,
            readOnly: !0,
            className: a.copyToClipboardInput,
            multiline: l,
            ...u,
            size: r,
            testId: s,
            endAdornment: /* @__PURE__ */ B.jsx(S0, { position: "end", children: i })
          }
        ) }),
        o === "card" && /* @__PURE__ */ B.jsx(ve, { className: e.copyToClipboardCard, children: n }),
        /* @__PURE__ */ B.jsx(
          ve,
          {
            className: he({
              [e.copyIconWrap]: !0,
              [e.copyIconWrapCard]: o === "card"
            }),
            children: /* @__PURE__ */ B.jsx(
              S2,
              {
                value: c,
                size: r === "small" ? "tiny" : "small",
                testId: "copy-to-clipboard"
              }
            )
          }
        )
      ]
    }
  );
}, $s = ar(
  (t) => fr({
    tabCard: {
      padding: 0
    },
    fullHeight: {
      display: "flex",
      flexDirection: "column",
      height: "100%",
      width: "100%"
    },
    tabHeading: {
      borderBottom: `1px solid ${t.palette.grey[100]}`,
      paddingLeft: t.spacing(2)
    },
    tabBody: {
      flexGrow: 1,
      overflow: "auto",
      display: "flex",
      flexDirection: "column"
    },
    tabs: {
      minHeight: "initial",
      "& .MuiTabs-flexContainer": {
        gap: t.spacing(3)
      }
    },
    tab: {
      padding: 0,
      minWidth: "auto",
      textTransform: "initial",
      minHeight: "initial",
      paddingTop: t.spacing(1),
      paddingBottom: t.spacing(1),
      color: t.palette.secondary.dark,
      fontWeight: 400,
      "&.Mui-selected": {
        color: t.palette.primary.main,
        fontWeight: 600
      },
      "&.Mui-disabled": {
        color: t.palette.grey[200]
      }
    },
    tabSmall: {
      fontSize: t.spacing(1.5),
      paddingTop: t.spacing(0.5),
      paddingBottom: t.spacing(0.5)
    },
    tabMedium: {
      fontSize: t.spacing(1.75),
      paddingTop: t.spacing(1),
      paddingBottom: t.spacing(1)
    },
    tabLarge: {
      fontSize: t.spacing(2),
      paddingBottom: t.spacing(1.25)
    },
    iconLeft: {
      "& .MuiTab-wrapper": {
        flexDirection: "row",
        "& .MuiSvgIcon-root": {
          marginBottom: 0,
          marginRight: t.spacing(1)
        }
      }
    },
    iconRight: {
      "& .MuiTab-wrapper": {
        flexDirection: "row-reverse",
        "& .MuiSvgIcon-root": {
          marginBottom: 0,
          marginLeft: t.spacing(1)
        }
      }
    },
    iconBottom: {
      "& .MuiTab-wrapper": {
        flexDirection: "column-reverse",
        "& .MuiSvgIcon-root": {
          marginBottom: 0,
          marginTop: t.spacing(1)
        }
      }
    },
    iconTop: {},
    tabBox: {
      display: "flex",
      alignItems: "center"
    },
    tabAction: {
      marginLeft: t.spacing(1)
    },
    tabActionsWrap: {
      display: "flex",
      width: "100%",
      justifyContent: "space-between"
    },
    tabActionsWrapCenter: {
      justifyContent: "center"
    },
    tabActions: {
      marginLeft: t.spacing(2),
      paddingBottom: t.spacing(1)
    },
    tabPanel: {
      paddingTop: t.spacing(2),
      paddingLeft: t.spacing(2),
      paddingRight: t.spacing(2),
      overflow: "visible",
      height: "100%",
      display: "flex",
      flexDirection: "column",
      boxSizing: "border-box"
    },
    tabPanelHide: {
      display: "none"
    },
    menuPaper: {
      border: `1px solid ${t.palette.grey[100]}`,
      borderRadius: 5
    },
    menuList: {
      paddingTop: 0,
      paddingBottom: 0
    },
    menuItemRoot: {
      "&:hover": {
        backgroundColor: "#E5E4E2"
      }
    },
    menuItemContent: {
      display: "flex",
      alignItems: "center",
      gridGap: t.spacing(1)
    },
    menuItemIcon: {
      display: "flex",
      alignItems: "center"
    },
    menuItemText: {}
  })
), T2 = (t) => {
  const {
    children: e,
    testId: a,
    size: r = "medium",
    actions: n,
    iconPosition: o = "left",
    ...i
  } = t, s = $s();
  return /* @__PURE__ */ B.jsxs(ve, { className: s.tabBox, children: [
    /* @__PURE__ */ B.jsx(
      Hv,
      {
        disableFocusRipple: !0,
        disableTouchRipple: !0,
        ...i,
        "data-cyid": a,
        className: he({
          [s.tab]: !0,
          [s.tabSmall]: r === "small",
          [s.tabMedium]: r === "medium",
          [s.tabLarge]: r === "large",
          [s.iconLeft]: o === "left",
          [s.iconRight]: o === "right",
          [s.iconBottom]: o === "bottom",
          [s.iconTop]: o === "top"
        }),
        children: e
      }
    ),
    n && /* @__PURE__ */ B.jsx(ve, { className: s.tabAction, children: n })
  ] });
};
function O2(t) {
  return {
    id: `simple-tab-${t}`,
    "aria-controls": `simple-tabpanel-${t}`
  };
}
const kc = (t) => {
  const {
    tabItems: e,
    children: a,
    handleChange: r,
    value: n,
    centered: o = !1,
    tabActions: i,
    title: s,
    testId: l,
    className: u,
    fullHeight: c,
    size: d = "medium"
  } = t, p = $s();
  return /* @__PURE__ */ B.jsxs(
    ve,
    {
      className: he(p.tabCard, u, {
        [p.fullHeight]: c
      }),
      children: [
        /* @__PURE__ */ B.jsxs(ve, { className: p.tabHeading, children: [
          s && /* @__PURE__ */ B.jsx(ve, { textAlign: o && "center", mb: 3, children: /* @__PURE__ */ B.jsx(Mt, { variant: "h2", gutterBottom: !0, children: s }) }),
          /* @__PURE__ */ B.jsxs(
            ve,
            {
              className: he(p.tabActionsWrap, {
                [p.tabActionsWrapCenter]: o
              }),
              children: [
                /* @__PURE__ */ B.jsx(
                  sb,
                  {
                    value: n,
                    onChange: r,
                    centered: o,
                    indicatorColor: "primary",
                    textColor: "primary",
                    className: p.tabs,
                    "data-cyid": l,
                    children: e.map((h, g) => {
                      const m = h.name.toLowerCase().replace(/\s+/g, "-");
                      return h.hidden ? null : h.buttonProps ? /* @__PURE__ */ B.jsx(
                        Br,
                        {
                          variant: "link",
                          startIcon: h.buttonProps.startIcon,
                          onClick: h.buttonProps.onClick,
                          color: "primary",
                          testId: `${l}-${m}`,
                          disabled: h.disabled,
                          children: h.name
                        },
                        h.name
                      ) : /* @__PURE__ */ Nd(
                        T2,
                        {
                          label: h.name,
                          ...O2(g),
                          key: h.name,
                          disabled: h.disabled,
                          actions: h.actions,
                          testId: `${l}-${m}`,
                          onClick: h.onClick,
                          size: d,
                          icon: h == null ? void 0 : h.icon,
                          iconPosition: h == null ? void 0 : h.iconPosition
                        }
                      );
                    })
                  }
                ),
                i && /* @__PURE__ */ B.jsx(ve, { className: p.tabActions, children: i })
              ]
            }
          )
        ] }),
        /* @__PURE__ */ B.jsxs(ve, { className: p.tabBody, children: [
          " ",
          a
        ] })
      ]
    }
  );
};
function Nn(t) {
  const { children: e, value: a, index: r, ...n } = t, o = $s();
  return /* @__PURE__ */ B.jsx(
    ve,
    {
      role: "tabpanel",
      hidden: a !== r,
      id: `simple-tabpanel-${r}`,
      "aria-labelledby": `simple-tab-${r}`,
      ...n,
      className: he(o.tabPanel, {
        [o.tabPanelHide]: a !== r
      }),
      children: a === r && e
    }
  );
}
const _2 = Ut.flushSync, P2 = "choreo-test-key", j2 = "choreo-oauth2-token", I2 = (t) => {
  const { token: e, apiEndpoint: a, sandboxEndpoint: r, topic: n, publish: o, subscribe: i, parameters: s, isDevportal: l, asyncType: u, payload: c } = t, d = gs(), [p, h] = jt(null), [g, m] = jt([]), [y, w] = jt(c || ""), [k, j] = jt(a), [f, x] = jt("production"), [E, L] = jt({}), [W, O] = jt(!1), [P, A] = jt("Connect"), [v, C] = jt(0), [M, D] = jt(e), [$, H] = jt("<your-header-here>"), [F, q] = jt("<your-header-value-here>"), [z, Z] = jt("http://example.com/callback"), [Q, ue] = jt("864000"), [de, Se] = jt("xxxxxxxxx"), fe = (ce, pe) => {
    C(pe);
  };
  function ye() {
    const ce = v === 0 ? "subscribe" : v === 1 ? "unsubscribe" : "subscribe";
    let pe = `curl -X POST ${k}`;
    return pe += " -H 'Content-Type: application/x-www-form-urlencoded'", pe += ` --data-urlencode 'hub.topic=${n}'`, pe += ` --data-urlencode 'hub.callback=${z}'`, pe += ` --data-urlencode 'hub.mode=${ce}'`, ce === "subscribe" && Q && Q.trim() && (pe += ` --data-urlencode 'hub.lease_seconds=${Q}'`), $ && $.trim() && F && F.trim() && (pe += ` -H '${$}: ${F}'`), pe;
  }
  pi(() => {
    const ce = f === "production" ? a : r, pe = ce.endsWith("/") ? ce.slice(0, -1) : ce;
    if (u === xr.WEBSUB)
      j(`${pe}/hub`);
    else if (n === "/*")
      j(`${pe}/`);
    else if (n === "/" || n === "")
      j(pe);
    else {
      const X = n.startsWith("/") ? n : `/${n}`;
      j(`${pe}${X}`);
    }
    s != null && L(
      Object.keys(s).reduce((X, V) => (X[V] = "", X), {})
    );
  }, [a, r, f, n]);
  const be = (ce) => {
    const pe = ce.target.value;
    if (pe === "sandbox" && (!r || r === "")) {
      x("production");
      return;
    }
    x(pe);
  }, we = () => {
    u !== xr.WEBSUB && p && y && p.readyState === p.OPEN && (p.send(y), m((ce) => [
      ...ce,
      {
        message: `Sent: ${y}`,
        timestamp: (/* @__PURE__ */ new Date()).toString(),
        randomKey: ce.length
      }
    ]), w(""));
  }, Ee = () => {
    A("Connecting...");
    let ce = k;
    for (const V in E)
      ce = ce.replace(`{${V}}`, E[V]);
    const pe = [];
    l ? pe.push(j2) : pe.push(P2), M && M.trim() && pe.push(M);
    const X = new WebSocket(ce, pe);
    h(X), X.onopen = () => {
      O(!0), m((V) => [
        ...V,
        {
          message: `Connected to ${ce}`,
          timestamp: (/* @__PURE__ */ new Date()).toString(),
          randomKey: V.length
        }
      ]), A("Disconnect");
    }, X.onmessage = (V) => {
      _2(() => {
        m((U) => [
          ...U,
          {
            message: `Received: ${V.data}`,
            timestamp: (/* @__PURE__ */ new Date()).toString(),
            randomKey: U.length
          }
        ]);
      });
    }, X.onerror = () => {
      O(!1), m((V) => [
        ...V,
        {
          message: "Error while connecting to the server. Please check the endpoint and try again.",
          timestamp: (/* @__PURE__ */ new Date()).toString(),
          randomKey: V.length
        }
      ]), A("Connect");
    };
  }, ge = () => {
    A("Disconnecting..."), setTimeout(() => {
      A("Connect"), O(!1), p == null || p.close(), p != null && p.CLOSED && (m((ce) => [
        ...ce,
        { message: "Disconnected", timestamp: (/* @__PURE__ */ new Date()).toString() }
      ]), A("Connect"));
    }, 1e3);
  }, ke = () => {
    m([]);
  };
  function je(ce) {
    return ce.split("").reduce((pe, X, V) => pe + X.charCodeAt(0) * V, 1);
  }
  const Te = [
    d.logColor1,
    d.logColor2,
    d.logColor3,
    d.logColor4
  ], { current: He } = Ld(/* @__PURE__ */ new Map()), _e = (ce) => {
    if (He.has(ce))
      return He.get(ce) || "";
    const pe = Te[je(ce) % Te.length];
    return He.set(ce, pe), pe;
  }, Fe = (ce) => /* @__PURE__ */ B.jsx(
    Xb,
    {
      getColor: _e,
      logEntry: ce,
      timeZone: "UTC",
      onCopy: () => {
      },
      isExpanded: !1,
      isHeiglighted: !1,
      isExpandable: !1
    }
  );
  function We(ce) {
    switch (ce) {
      case "Connecting...":
        return "Connecting...";
      case "Connect":
        return "Connect";
      case "Disconnect":
        return "Disconnect";
      case "Disconnecting...":
        return "Disconnecting...";
      default:
        return "Connect";
    }
  }
  const Ue = Tc(
    () => P === "Connect" ? "Connect to the server to start sending messages." : "Enter your message to send.",
    [P]
  ), tt = [{ name: "Parameters" }, { name: "Payload" }, { name: "Headers" }], Qe = [{ name: "Payload" }, { name: "Headers" }], Ve = [{ name: "Subscribe" }, { name: "Unsubscribe" }];
  return u === xr.WEBSUB ? /* @__PURE__ */ B.jsx(ve, { className: d.topicContainer, children: /* @__PURE__ */ B.jsxs(Xl, { square: !0, testId: "topic-accordion", bordered: !0, children: [
    /* @__PURE__ */ B.jsx(
      Zl,
      {
        "aria-controls": "panel1a-content",
        testId: "topic-accordion-summary",
        children: /* @__PURE__ */ B.jsxs(ve, { className: d.topicTypeContainer, children: [
          /* @__PURE__ */ B.jsx(ve, { className: d.typeChipContainer, children: /* @__PURE__ */ B.jsx(
            ii,
            {
              label: "SUB",
              testId: "default",
              variant: "contained",
              color: "success",
              size: "medium",
              disabled: !i
            }
          ) }),
          /* @__PURE__ */ B.jsx(ve, { className: d.typeChipContainer, children: /* @__PURE__ */ B.jsx(Mt, { variant: "body2", children: n }) })
        ] })
      }
    ),
    /* @__PURE__ */ B.jsx(Jl, { testId: "topic-accordion-details", children: /* @__PURE__ */ B.jsx(ve, { width: "100%", style: { maxWidth: "100%", boxSizing: "border-box", overflow: "hidden" }, children: n && /* @__PURE__ */ B.jsxs(
      fi,
      {
        fallback: /* @__PURE__ */ B.jsx(
          ve,
          {
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            children: /* @__PURE__ */ B.jsx(Xn, { size: 25 })
          }
        ),
        children: [
          /* @__PURE__ */ B.jsxs(ve, { className: d.endpointContainer, children: [
            /* @__PURE__ */ B.jsx(
              wl,
              {
                size: "small",
                variant: "outlined",
                style: {
                  width: 150,
                  flexShrink: 0,
                  height: 40
                },
                children: /* @__PURE__ */ B.jsxs(
                  Bl,
                  {
                    value: f,
                    onChange: be,
                    disabled: W,
                    "data-testid": "endpoint-type-select",
                    inputProps: { "aria-label": "Endpoint type" },
                    style: {
                      height: 40,
                      borderRadius: 4,
                      backgroundColor: "#ffffff",
                      boxShadow: "0 1px 3px rgba(0, 0, 0, 0.08)"
                    },
                    children: [
                      /* @__PURE__ */ B.jsx(xa, { value: "production", children: "Production" }),
                      /* @__PURE__ */ B.jsx(xa, { value: "sandbox", disabled: !r || r === "", children: "Sandbox" })
                    ]
                  }
                )
              }
            ),
            /* @__PURE__ */ B.jsx(ve, { className: d.textInput, children: /* @__PURE__ */ B.jsx(
              Ea,
              {
                value: k,
                size: "small",
                testId: "endpoint-url"
              }
            ) }),
            u !== xr.WEBSUB && /* @__PURE__ */ B.jsx(
              Br,
              {
                color: W ? "secondary" : "primary",
                variant: "contained",
                pill: !1,
                size: "small",
                disabled: u === xr.WEBSUB,
                testId: "disconnect",
                style: {
                  flexShrink: 0,
                  height: 40,
                  minWidth: 120,
                  fontWeight: 500,
                  fontSize: 14,
                  textTransform: "none",
                  borderRadius: 4
                },
                onClick: () => W ? ge() : Ee(),
                children: We(P)
              }
            )
          ] }),
          /* @__PURE__ */ B.jsx(ve, { style: { width: "100%", maxWidth: "100%", boxSizing: "border-box" }, children: /* @__PURE__ */ B.jsx(ve, { className: d.tabs, children: /* @__PURE__ */ B.jsxs(ve, { className: d.tabPanelContent, children: [
            /* @__PURE__ */ B.jsx(
              Mt,
              {
                "data-testid": "token-heading",
                variant: "body1",
                className: d.label,
                display: "block",
                children: "Token"
              }
            ),
            /* @__PURE__ */ B.jsx(ve, { className: d.parameterContainerWrapper, style: { height: "auto", overflow: "visible" }, children: /* @__PURE__ */ B.jsxs(
              ve,
              {
                className: d.parameterContainer,
                style: { height: "auto", overflow: "visible" },
                children: [
                  /* @__PURE__ */ B.jsx(
                    yr,
                    {
                      testId: "input-headers",
                      className: d.callbackURLTextInput,
                      type: "text",
                      placeholder: "Header",
                      label: "Header",
                      value: $,
                      onChange: (ce) => {
                        H(ce.target.value);
                      },
                      error: $ === "",
                      helperText: "Enter Header"
                    }
                  ),
                  /* @__PURE__ */ B.jsx(
                    yr,
                    {
                      testId: "input-header-value",
                      className: d.callbackURLTextInput,
                      type: "text",
                      placeholder: "Value",
                      label: "Value",
                      value: F,
                      onChange: (ce) => {
                        q(ce.target.value);
                      },
                      error: F === "",
                      helperText: "Enter Value"
                    }
                  )
                ]
              },
              "api-token"
            ) })
          ] }) }) }),
          /* @__PURE__ */ B.jsx(ve, { style: { width: "100%", maxWidth: "100%", boxSizing: "border-box" }, children: /* @__PURE__ */ B.jsxs(
            kc,
            {
              className: d.tabs,
              tabItems: Ve,
              handleChange: fe,
              value: v,
              testId: "env-tab-card",
              fullHeight: !0,
              children: [
                /* @__PURE__ */ B.jsx(
                  Nn,
                  {
                    value: v,
                    index: 0,
                    children: /* @__PURE__ */ B.jsxs(ve, { className: d.tabPanelContent, children: [
                      /* @__PURE__ */ B.jsx(ve, { className: d.parameterContainerWrapper, children: /* @__PURE__ */ B.jsxs(ve, { className: d.parameterContainer, children: [
                        /* @__PURE__ */ B.jsx(
                          yr,
                          {
                            testId: "input-callbackurl",
                            className: d.callbackURLTextInput,
                            type: "text",
                            placeholder: "Callback URL",
                            label: "Callback URL",
                            value: z,
                            onChange: (ce) => {
                              const pe = ce.target.value;
                              Z(pe);
                            },
                            helperText: "Enter Callback URL"
                          }
                        ),
                        /* @__PURE__ */ B.jsx(
                          yr,
                          {
                            testId: "input-leaseseconds",
                            className: d.callbackURLTextInput,
                            type: "text",
                            placeholder: "Lease Seconds",
                            label: "Lease Seconds",
                            value: Q,
                            onChange: (ce) => {
                              const pe = ce.target.value;
                              ue(pe);
                            },
                            helperText: "Enter Lease Seconds"
                          }
                        )
                      ] }) }),
                      /* @__PURE__ */ B.jsxs(kl, { children: [
                        /* @__PURE__ */ B.jsx(
                          Mt,
                          {
                            "data-testid": "curl-command-txt",
                            variant: "body1",
                            className: d.label,
                            display: "block",
                            children: "Command"
                          }
                        ),
                        /* @__PURE__ */ B.jsx(ve, { className: d.payloadContainer, children: /* @__PURE__ */ B.jsx(
                          Ea,
                          {
                            value: ye(),
                            size: "small",
                            testId: "curl-command",
                            isMultiline: !0
                          }
                        ) })
                      ] })
                    ] })
                  }
                ),
                /* @__PURE__ */ B.jsx(
                  Nn,
                  {
                    value: v,
                    index: 1,
                    children: /* @__PURE__ */ B.jsxs(ve, { className: d.tabPanelContent, children: [
                      /* @__PURE__ */ B.jsx(ve, { className: d.parameterContainerWrapper, children: /* @__PURE__ */ B.jsx(ve, { className: d.parameterContainer, children: /* @__PURE__ */ B.jsx(
                        yr,
                        {
                          testId: "input-callbackurl",
                          className: d.callbackURLTextInput,
                          type: "text",
                          placeholder: "Callback URL",
                          label: "Callback URL",
                          value: z,
                          onChange: (ce) => {
                            const pe = ce.target.value;
                            Z(pe);
                          },
                          helperText: "Enter Callback URL"
                        }
                      ) }) }),
                      /* @__PURE__ */ B.jsxs(kl, { children: [
                        /* @__PURE__ */ B.jsx(
                          Mt,
                          {
                            "data-testid": "curl-command-txt",
                            variant: "body1",
                            className: d.label,
                            display: "block",
                            children: "Command"
                          }
                        ),
                        /* @__PURE__ */ B.jsx(ve, { className: d.payloadContainer, children: /* @__PURE__ */ B.jsx(
                          Ea,
                          {
                            value: ye(),
                            size: "small",
                            testId: "curl-command",
                            isMultiline: !0
                          }
                        ) })
                      ] })
                    ] })
                  }
                )
              ]
            }
          ) })
        ]
      }
    ) }) })
  ] }) }) : /* @__PURE__ */ B.jsx(ve, { className: d.topicContainer, children: /* @__PURE__ */ B.jsxs(Xl, { square: !0, testId: "topic-accordion", bordered: !0, children: [
    /* @__PURE__ */ B.jsx(
      Zl,
      {
        "aria-controls": "panel1a-content",
        testId: "topic-accordion-summary",
        children: /* @__PURE__ */ B.jsxs(ve, { className: d.topicTypeContainer, children: [
          u === xr.WS && /* @__PURE__ */ B.jsx(ve, { className: d.typeChipContainer, children: /* @__PURE__ */ B.jsx(
            ii,
            {
              label: "PUB",
              testId: "default",
              variant: "contained",
              color: "info",
              size: "medium",
              disabled: !o
            }
          ) }),
          /* @__PURE__ */ B.jsx(ve, { className: d.typeChipContainer, children: /* @__PURE__ */ B.jsx(
            ii,
            {
              label: "SUB",
              testId: "default",
              variant: "contained",
              color: "success",
              size: "medium",
              disabled: !i
            }
          ) }),
          /* @__PURE__ */ B.jsx(ve, { className: d.typeChipContainer, children: /* @__PURE__ */ B.jsx(Mt, { variant: "body2", children: n }) })
        ] })
      }
    ),
    /* @__PURE__ */ B.jsx(Jl, { testId: "topic-accordion-details", children: /* @__PURE__ */ B.jsx(ve, { width: "100%", style: { maxWidth: "100%", boxSizing: "border-box", overflow: "hidden" }, children: n && /* @__PURE__ */ B.jsxs(
      fi,
      {
        fallback: /* @__PURE__ */ B.jsx(
          ve,
          {
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            children: /* @__PURE__ */ B.jsx(Xn, { size: 25 })
          }
        ),
        children: [
          u === xr.WS && /* @__PURE__ */ B.jsxs(ve, { className: d.endpointContainer, children: [
            /* @__PURE__ */ B.jsx(
              wl,
              {
                size: "small",
                variant: "outlined",
                style: {
                  width: 150,
                  flexShrink: 0,
                  height: 40
                },
                children: /* @__PURE__ */ B.jsxs(
                  Bl,
                  {
                    value: f,
                    onChange: be,
                    disabled: W,
                    "data-testid": "endpoint-type-select",
                    inputProps: { "aria-label": "Endpoint type" },
                    style: {
                      height: 40,
                      borderRadius: 4,
                      backgroundColor: "#ffffff",
                      boxShadow: "0 1px 3px rgba(0, 0, 0, 0.08)"
                    },
                    children: [
                      /* @__PURE__ */ B.jsx(xa, { value: "production", children: "Production" }),
                      /* @__PURE__ */ B.jsx(xa, { value: "sandbox", disabled: !r || r === "", children: "Sandbox" })
                    ]
                  }
                )
              }
            ),
            /* @__PURE__ */ B.jsx(ve, { className: d.textInput, children: /* @__PURE__ */ B.jsx(
              Ea,
              {
                value: k,
                size: "small",
                testId: "endpoint-url"
              }
            ) }),
            /* @__PURE__ */ B.jsx(
              Br,
              {
                color: W ? "secondary" : "primary",
                variant: "contained",
                pill: !1,
                size: "small",
                testId: "disconnect",
                style: {
                  flexShrink: 0,
                  height: 40,
                  minWidth: 120,
                  fontWeight: 500,
                  fontSize: 14,
                  textTransform: "none",
                  borderRadius: 4
                },
                onClick: () => W ? ge() : Ee(),
                children: We(P)
              }
            )
          ] }),
          /* @__PURE__ */ B.jsxs(ve, { style: { width: "100%", maxWidth: "100%", boxSizing: "border-box" }, children: [
            /* @__PURE__ */ B.jsxs(
              kc,
              {
                className: d.tabs,
                tabItems: Object.keys(E).length === 0 ? Qe : tt,
                handleChange: fe,
                value: v,
                testId: "env-tab-card",
                fullHeight: !0,
                children: [
                  Object.keys(E).length !== 0 && /* @__PURE__ */ B.jsx(Nn, { value: v, index: 0, children: /* @__PURE__ */ B.jsx(ve, { className: d.tabPanelContent, children: /* @__PURE__ */ B.jsxs(ve, { className: d.parameterContainerWrapper, children: [
                    /* @__PURE__ */ B.jsx(ve, { className: d.parameterContainer, children: Object.keys(E).map((ce) => /* @__PURE__ */ B.jsx(ve, { children: /* @__PURE__ */ B.jsx(
                      yr,
                      {
                        testId: `input-${ce}`,
                        className: d.paramTextInput,
                        type: "text",
                        placeholder: ce,
                        label: ce,
                        value: E[ce],
                        onChange: (pe) => {
                          const X = {
                            ...E,
                            [ce]: pe.target.value
                          };
                          L(X);
                        },
                        error: E[ce] === "",
                        helperText: "Enter parameter value"
                      }
                    ) }, ce)) }),
                    /* @__PURE__ */ B.jsx(ve, { className: d.executeButton, children: /* @__PURE__ */ B.jsx(
                      Br,
                      {
                        testId: "execute",
                        disabled: W,
                        variant: "outlined",
                        onClick: Ee,
                        startIcon: /* @__PURE__ */ B.jsx(Kb, {}),
                        size: "small",
                        children: /* @__PURE__ */ B.jsx(Mt, { variant: "body1", align: "center", children: "Execute" })
                      }
                    ) })
                  ] }) }) }),
                  /* @__PURE__ */ B.jsx(
                    Nn,
                    {
                      value: v,
                      index: Object.keys(E).length === 0 ? 0 : 1,
                      children: /* @__PURE__ */ B.jsxs(ve, { className: d.tabPanelContent, children: [
                        /* @__PURE__ */ B.jsx(ve, { className: d.payloadContainer, children: /* @__PURE__ */ B.jsx(
                          ny,
                          {
                            fileContent: y,
                            language: "json",
                            height: "170px",
                            width: "100%",
                            setFileContent: w
                          }
                        ) }),
                        /* @__PURE__ */ B.jsx(ve, { mt: 5, children: /* @__PURE__ */ B.jsx(ms, { title: Ue, placement: "top-start", children: /* @__PURE__ */ B.jsx(ve, { className: d.sendIcon, children: /* @__PURE__ */ B.jsx(
                          Br,
                          {
                            testId: "payload-send",
                            disabled: y.length === 0 || !W,
                            variant: "outlined",
                            startIcon: /* @__PURE__ */ B.jsx(qb, {}),
                            onClick: we,
                            size: "small",
                            children: /* @__PURE__ */ B.jsx(Mt, { variant: "body1", align: "center", children: "Send" })
                          }
                        ) }) }) })
                      ] })
                    }
                  ),
                  /* @__PURE__ */ B.jsx(
                    Nn,
                    {
                      value: v,
                      index: Object.keys(E).length === 0 ? 1 : 2,
                      children: /* @__PURE__ */ B.jsx(ve, { className: d.tabPanelContent, children: /* @__PURE__ */ B.jsx(ve, { className: d.parameterContainerWrapper, children: /* @__PURE__ */ B.jsx(
                        ve,
                        {
                          className: d.parameterContainer,
                          children: /* @__PURE__ */ B.jsx(
                            yr,
                            {
                              testId: "input-headers",
                              className: d.apiTokenTextInput,
                              type: "text",
                              placeholder: "API Token",
                              label: "API Token",
                              value: M,
                              onChange: (ce) => {
                                const pe = ce.target.value;
                                D(pe);
                              },
                              error: M === "",
                              helperText: "Enter API Token"
                            }
                          )
                        },
                        "api-token"
                      ) }) })
                    }
                  )
                ]
              }
            ),
            /* @__PURE__ */ B.jsx(
              ey,
              {
                messages: g,
                clearLogs: ke,
                isDisabled: g.length < 1
              }
            ),
            /* @__PURE__ */ B.jsx(ve, { children: /* @__PURE__ */ B.jsx(ve, { className: d.outputContainer, children: g.map(
              (ce, pe) => Fe({
                timestamp: g[pe].timestamp,
                message: g[pe].message,
                randomKey: pe,
                isNewLog: !1
              })
            ) }) })
          ] })
        ]
      }
    ) }) })
  ] }) });
};
var xr = /* @__PURE__ */ ((t) => (t.WS = "WS", t.WEBSUB = "WEBSUB", t))(xr || {});
function ka(t) {
  const { channels: e } = t;
  return Object.entries(e).map(
    ([r, n]) => ({
      name: r,
      subscribe: !!(n != null && n.subscribe),
      publish: !!(n != null && n.publish),
      parameters: n.parameters
    })
  );
}
function N2(t) {
  const {
    token: e = "",
    apiEndpoint: a = "",
    sandboxEndpoint: r = "",
    asyncapi: n = {
      asyncapi: "2.0.0",
      info: {
        title: "WebSocket API",
        version: "1.0.0"
      },
      channels: {
        "/": {
          subscribe: {
            message: "A sample message",
            operationId: "test"
          },
          publish: {
            message: "A sample message",
            operationId: "test"
          }
        }
      }
    },
    isDevportal: o = !1,
    asyncApiType: i = "WS"
    /* WS */
  } = t, [s, l] = jt([
    { name: "/*", subscribe: !0, publish: !0, parameters: {} }
  ]);
  pi(() => {
    n != null && n.channels && l(ka(n));
  }, [n]);
  const u = Tc(() => {
    var d, p;
    if (n && Object.keys(n).length > 0 && a) {
      let h = Vl(n);
      return (d = h.asyncapi) != null && d.startsWith("2") ? h = {
        ...h,
        servers: {
          default: { url: a, protocol: "ws" },
          sandbox: { url: r, protocol: "ws" }
        }
      } : (p = h.asyncapi) != null && p.startsWith("3") && (h = {
        ...h,
        servers: {
          default: { url: a, protocol: "ws" },
          sandbox: { url: r, protocol: "ws" }
        }
      }), h != null && h.channels && l(ka(h)), h;
    } else {
      let h = Vl(n);
      l(ka(h));
    }
    return null;
  }, [n, a, r]);
  pi(() => {
    u != null && u.channels && l(ka(u));
  }, [u]);
  function c(d) {
    return i === "WEBSUB" ? JSON.stringify({
      "hub.mode": "subscribe",
      "hub.topic": d || "sample-topic",
      "hub.callback": "http://example.com/callback",
      "hub.secret": "xxxxxxxxx",
      "hub.lease_seconds": 864e3
    }) : '{ "message": "Hello Server" }';
  }
  return !s || !(n != null && n.channels) ? /* @__PURE__ */ B.jsx(
    ve,
    {
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      padding: 8,
      children: /* @__PURE__ */ B.jsx(Xn, { size: 25 })
    }
  ) : a ? /* @__PURE__ */ B.jsx(ve, { children: s.map(({ name: d, publish: p, subscribe: h, parameters: g }) => /* @__PURE__ */ B.jsx(
    I2,
    {
      token: e,
      apiEndpoint: a,
      sandboxEndpoint: r || "",
      topic: d,
      publish: p,
      subscribe: h,
      parameters: g,
      isDevportal: o,
      payload: c(d),
      asyncType: i || "WS"
      /* WS */
    },
    d
  )) }) : /* @__PURE__ */ B.jsx(B.Fragment, {});
}
export {
  N2 as WebSocketViewer,
  N2 as default
};
