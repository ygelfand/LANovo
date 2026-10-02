vec3 scene(highp vec2 p, highp vec2 uv) {
	vec3 cA = vec3(u[0], u[1], u[2]), cB = vec3(u[3], u[4], u[5]);
	highp float cx = u[6], cy = u[7], rg = u[8];
	float lv = u[9], flash = u[10];
	vec3 col = texture2D(t0, uv).rgb;
	highp float d = length(p - vec2(cx, cy));
	float t = float(d / rg);
	if (t < 1.0) {
		vec4 h = t < 0.6 ? mix(vec4(cA, 0.16 + 0.22 * lv + 0.2 * flash), vec4(cB, 0.06 + 0.1 * lv), t / 0.6)
			: mix(vec4(cB, 0.06 + 0.1 * lv), vec4(cB, 0.02), (t - 0.6) / 0.4);
		col += h.rgb * h.a;
	}
	col += linesAt(vgl).rgb;
	highp float cr = u[11] * 2.2;
	float c = float(d / cr);
	if (c < 1.0) {
		vec4 k = c < 0.3 ? mix(vec4(1.0, 1.0, 1.0, 0.95), vec4(mix(cA, vec3(1.0), 0.5), 0.8), c / 0.3)
			: mix(vec4(mix(cA, vec3(1.0), 0.5), 0.8), vec4(cA, 0.0), (c - 0.3) / 0.7);
		col += k.rgb * k.a;
	}
	col = min(col, 1.0);
	vec4 g = texture2D(t1, uv);
	col = g.rgb + col * (1.0 - g.a);
	return min(col, 1.0);
}

void main() {
	highp float s = res.x / u[14];
	highp vec2 p = pixel() / s;
#ifdef LIGHT
	gl_FragColor = vec4(scene(p, vtex), 1.0);
#else
	gl_FragColor = vec4(min(scene(p, vtex) + glow(), 1.0), 1.0);
#endif
}
