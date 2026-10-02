#ifdef SPLAT
void main() {
	vec2 d = gl_PointCoord * 2.0 - 1.0;
	float d2 = dot(d, d);
	if (d2 >= 1.0) discard;
	float q = 1.0 - d2;
	float c = q * q * q;
	float w = c * c;
#ifdef SPLAT_A
	gl_FragColor = vec4(c, c * vcol.a, w, 0.0);
#else
	gl_FragColor = vec4(w * vcol.rgb, 0.0);
#endif
}
#else

highp float height(highp vec4 a) {
	return a.r > 0.5 ? sqrt(a.r - 0.5) * 1.35 * (a.g / a.r) : 0.0;
}

vec4 surface(highp vec2 uv) {
	highp vec4 a = texture2D(splatA, uv);
	if (a.r < 0.3) return vec4(0.0);
	highp vec2 dx = vec2(u[1], 0.0), dy = vec2(0.0, u[2]);
	highp vec4 ax1 = texture2D(splatA, uv + dx), ax0 = texture2D(splatA, uv - dx);
	highp vec4 ay1 = texture2D(splatA, uv - dy), ay0 = texture2D(splatA, uv + dy);
	highp float gx = (ax1.r - ax0.r) * 0.5, gy = (ay1.r - ay0.r) * 0.5;
	highp float gm = sqrt(gx * gx + gy * gy) + 1e-6;
	float al = clamp(float((a.r - 0.5) / gm + 0.5), 0.0, 1.0);
	if (al <= 0.0) return vec4(0.0);
	highp float hx = (height(ax1) - height(ax0)) * 0.5, hy = (height(ay1) - height(ay0)) * 0.5;
	highp float il = inversesqrt(hx * hx + hy * hy + 1.0);
	vec3 n = vec3(-hx * il, -hy * il, il);
	vec3 pc = texture2D(splatB, uv).rgb / max(float(a.b), 1e-6);
	vec3 L = vec3(-0.47, -0.62, 0.63);
	vec3 hv = normalize(L + vec3(0.0, 0.0, 1.0));
	float dif = max(dot(n, L), 0.0);
	float sp = max(dot(n, hv), 0.0);
	float s2 = sp * sp, s4 = s2 * s2, s8 = s4 * s4, s16 = s8 * s8;
	float spec = s16 * s16 * 235.0 / 255.0;
	float edge = 1.0 - n.z;
	float shade = (0.42 + 0.72 * dif) * (1.0 - 0.55 * edge * edge);
	return vec4(min(pc * shade + spec, 1.0) * al, al);
}

void main() {
#ifdef FEED
	gl_FragColor = texture2D(feed, vgl) * u[0];
#else
#ifdef LIGHT
	gl_FragColor = vec4(surface(vgl).rgb, 1.0);
#else
	highp vec2 p = pixel();
	float d = float(length(p - vec2(u[3], u[4])) / u[5]);
	vec3 col = mix(vec3(21.0, 21.0, 27.0), vec3(5.0, 5.0, 7.0), min(d, 1.0)) / 255.0;
	vec4 st = texture2D(feed, vgl);
	col = col * (1.0 - 0.5 * st.a) + 0.5 * st.rgb;
	vec4 s = surface(vgl);
	col = s.rgb + col * (1.0 - s.a);
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
#endif
}
#endif
