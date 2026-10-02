#define WAVE(kx, ky, w, a) { float ph = dot(q, vec2(kx, ky)) + t * (w); float c = cos(ph) * (a); g += vec2(kx, ky) * c; }

#ifdef FEED
void main() {
	highp vec2 tx = 1.0 / res;
	vec4 here = texture2D(feed, vgl);
	vec2 q = vec2(vgl * vec2(res.x / res.y, 1.0) * 6.2831);
	float t = mod(u[2], 628.3);
	vec2 g = vec2(0.0);
	WAVE(1.3, 0.7, 0.21, 1.0)
	WAVE(-0.9, 1.6, -0.17, 0.8)
	WAVE(2.9, -1.7, 0.33, 0.35)
	WAVE(-2.3, -3.1, 0.27, 0.28)
	WAVE(4.7, 3.9, -0.41, 0.12)
	vec2 vel = vec2(g.y, -g.x) * u[3];
	vel.y -= 0.5 + 1.8 * min(here.a, 1.5);
	vec4 s = texture2D(feed, vgl - vel * tx);
	gl_FragColor = max(s * u[4] - s * (0.003 / max(s.a, 0.05)), 0.0);
}
#else
void main() {
	highp vec2 tx = 2.5 / res;
	vec4 ink = texture2D(feed, vgl);
	float dx = texture2D(feed, vgl + vec2(tx.x, 0.0)).a - ink.a;
	float dy = texture2D(feed, vgl + vec2(0.0, tx.y)).a - ink.a;
	float y = float(vgl.y);
	vec3 water = vec3(0.012, 0.025, 0.045) + vec3(0.02, 0.05, 0.08) * y * y;
	water += vec3(0.03, 0.06, 0.08) * smoothstep(0.5, 1.0, y) * (0.6 + 0.4 * sin(float(vgl.x) * 9.0 + u[2] * 0.4));
	float sheen = clamp(0.9 + 1.6 * (dy - 0.4 * dx), 0.55, 1.4);
	vec3 lit = (1.0 - exp(-ink.rgb * 2.6)) * sheen * (0.75 + 0.35 * y);
	vec3 col = water * exp(-ink.a * 0.4) + lit;
#ifdef LIGHT
	gl_FragColor = vec4(lit * 0.45, 1.0);
#else
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
#endif
