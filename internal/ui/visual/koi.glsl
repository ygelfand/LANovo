highp float hash(highp vec2 q) {
	highp vec3 p3 = fract(vec3(q.xyx) * 0.1031);
	p3 += dot(p3, p3.yzx + 33.33);
	return fract((p3.x + p3.y) * p3.z);
}

float vnoise(highp vec2 q) {
	highp vec2 i = floor(q);
	vec2 f = vec2(fract(q));
	f = f * f * (3.0 - 2.0 * f);
	float a = float(hash(i)), b = float(hash(i + vec2(1.0, 0.0)));
	float c = float(hash(i + vec2(0.0, 1.0))), d = float(hash(i + vec2(1.0, 1.0)));
	return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

#ifdef FEED
#define DROP(o) { highp vec2 dd = (vtex - vec2(u[o], u[o + 1])) * vec2(u[0], u[1]); float k = max(0.0, 1.0 - float(dot(dd, dd)) / max(u[o + 2] * u[o + 2], 1.0)); n += u[o + 3] * k * k; }
void main() {
	highp vec2 tx = 1.0 / res;
	vec4 c = texture2D(feed, vgl);
	float hr = texture2D(feed, vgl + vec2(tx.x, 0.0)).r, hl = texture2D(feed, vgl - vec2(tx.x, 0.0)).r;
	float hu = texture2D(feed, vgl + vec2(0.0, tx.y)).r, hd = texture2D(feed, vgl - vec2(0.0, tx.y)).r;
	float n = (hr + hl + hu + hd) * 0.5 - c.g;
	n *= 0.986;
	DROP(10) DROP(14) DROP(18) DROP(22)
	gl_FragColor = vec4(n, c.r, hr - hl, hu - hd);
}
#elif defined(SPLAT)

float koiProfile(float t) {
	if (t < 0.3) return 0.7 + 0.3 * smoothstep(0.0, 0.3, t);
	return 1.0 - 0.62 * smoothstep(0.3, 1.0, t);
}

vec3 koiColor(float breed, float along, float side, float seed) {
	highp vec2 bq = vec2(along * 7.0, side * 1.3 + seed * 13.0);
	float n1 = vnoise(bq);
	float n2 = vnoise(bq * 2.1 + 5.0);
	vec3 white = vec3(0.96, 0.94, 0.9), red = vec3(0.92, 0.28, 0.08), black = vec3(0.07, 0.07, 0.08);
	vec3 c;
	if (breed < 0.5) c = n1 > 0.52 ? red : white;
	else if (breed < 1.5) c = n2 > 0.78 ? black : (n1 > 0.55 ? red : white);
	else if (breed < 2.5) c = mix(vec3(1.0, 0.72, 0.25), vec3(1.0, 0.9, 0.55), n2);
	else if (breed < 3.5) c = n1 > 0.5 ? black : (n2 > 0.55 ? red : white);
	else c = mix(vec3(0.55, 0.38, 0.22), vec3(0.72, 0.52, 0.3), n1);
	if (along < 0.1) c = mix(c, white * 0.9, 0.35);
	return c;
}

void main() {
	vec2 d = gl_PointCoord * 2.0 - 1.0;
	float r = length(d);
	if (r >= 1.0) discard;
	float edge = clamp((1.0 - r) * vsize * 0.5 + 0.5, 0.0, 1.0);
#ifdef SPLAT_A
	if (vcol.a < 0.25) discard;
	if (vcol.a < 0.75) {
		float q = 1.0 - r * r;
		float w = q * 0.42 * (1.0 - 0.5 * vcol.r);
		gl_FragColor = vec4(vec3(1.0, 0.86, 0.76) * w, w);
		return;
	}
	float a = (vcol.b - 0.5) * 6.2832;
	vec2 dir = vec2(cos(a), sin(a));
	float pr = koiProfile(vcol.r);
	float along = vcol.r + dot(d, dir) * 0.15 * pr;
	float side = (d.x * -dir.y + d.y * dir.x) * pr;
	float breed = floor(vcol.g), seed = fract(vcol.g) / 0.9;
	vec3 c = koiColor(breed, along, side, seed) * (0.82 + 0.2 * (1.0 - abs(side)));
	gl_FragColor = vec4(c * edge, edge);
#else
	if (vcol.a > 0.25) discard;
	float q = 1.0 - r * r;
	gl_FragColor = vec4(q * q, 0.0, 0.0, 0.0);
#endif
}
#else

void main() {
	highp vec2 p = pixel() * (u[0] / res.x);
	vec2 g = texture2D(feed, vgl).ba;
	vec3 n = normalize(vec3(-g * 6.0, 1.0));
	float spec = max(dot(n, normalize(vec3(-0.35, 0.55, 1.0))), 0.0);
	spec *= spec; spec *= spec; spec *= spec; spec *= spec; spec *= spec; spec *= spec;
	vec3 glint = vec3(0.95, 0.97, 1.0) * spec * u[6];
#ifdef LIGHT
	gl_FragColor = vec4(glint, 1.0);
#else
	highp vec2 q = (p + g * u[4]) / vec2(u[0], u[1]);
	vec3 bed = texture2D(t0, q).rgb;
	bed *= 0.92 + clamp(0.5 - length(g) * 3.0, -0.2, 0.5) * 0.3;
	bed *= 1.0 - 0.4 * clamp(texture2D(splatB, vec2(q.x, 1.0 - q.y)).r * 0.9, 0.0, 1.0);
	vec3 col = bed * vec3(0.6, 0.82, 0.76);
	vec4 f = texture2D(splatA, vec2(q.x, 1.0 - q.y));
	col = mix(col, f.rgb / max(f.a, 1e-3) * vec3(0.85, 0.95, 0.9), clamp(f.a, 0.0, 1.0) * 0.92);
	vec4 pad = texture2D(t1, vtex);
	col = pad.rgb + col * (1.0 - pad.a);
	col += glint;
	vec2 vq = vec2(vtex) * 2.0 - 1.0;
	col *= 1.0 - 0.3 * dot(vq, vq);
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
#endif
