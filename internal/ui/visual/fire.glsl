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

vec3 flame(float h) {
	h = clamp(h, 0.0, 1.2);
	vec3 c = mix(vec3(0.0), vec3(0.55, 0.05, 0.01), smoothstep(0.02, 0.25, h));
	c = mix(c, vec3(1.0, 0.38, 0.05), smoothstep(0.2, 0.55, h));
	c = mix(c, vec3(1.0, 0.8, 0.35), smoothstep(0.5, 0.85, h));
	return mix(c, vec3(1.0, 0.97, 0.85), smoothstep(0.85, 1.15, h));
}

#ifdef FEED
void main() {
	highp vec2 tx = 1.0 / res;
	highp float row = floor(vgl.y * res.y);
	float j = sin(float(vgl.x) * 37.0 + float(row) * 0.06 + u[2] * 2.7) * 1.3 + sin(float(vgl.x) * 11.0 - float(row) * 0.021 - u[2] * 1.6) * 1.0;
	float rise = 2.0 + 3.0 * vnoise(vec2(float(vgl.x) * 9.0, u[2] * 0.7));
	highp vec2 at = vgl + vec2(j * tx.x, -rise * tx.y);
	float h = texture2D(feed, at).r * 0.5 + (texture2D(feed, at - vec2(tx.x, 0.0)).r + texture2D(feed, at + vec2(tx.x, 0.0)).r) * 0.25;
	float n = vnoise(vec2(vgl.x * 14.0, vgl.y * 6.0 - u[2] * 1.8));
	h -= (0.0025 + 0.035 * n * n * n) * max(0.45, 2.0 - 1.45 * u[3]);
	float x = float(vgl.x);
	float tongues = 0.5 + 0.5 * sin(x * 17.0 + u[2] * 1.7) * sin(x * 6.3 - u[2] * 1.1 + 1.3);
	float src = step(float(vgl.y), 0.012 + 0.035 * tongues) * (0.45 + 0.5 * min(u[3], 1.2)) * (0.6 + 0.4 * tongues) * (0.9 + 0.2 * float(hash(vec2(floor(x * res.x * 0.2), floor(u[2] * 15.0)))));
	gl_FragColor = vec4(max(h, src), 0.0, 0.0, 1.0);
}
#else
void main() {
	float h = texture2D(feed, vgl).r;
	vec3 fire = flame(h);
	float bed = smoothstep(0.06, 0.0, float(vgl.y));
	vec3 embers = vec3(0.9, 0.25, 0.04) * bed * (0.5 + 0.5 * u[3]);
#ifdef LIGHT
	gl_FragColor = vec4(fire + embers, 1.0);
#else
	vec3 col = fire + embers + vec3(0.05, 0.012, 0.0) * (1.0 - float(vgl.y)) * u[3];
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
#endif
