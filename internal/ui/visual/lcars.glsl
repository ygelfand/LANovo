vec3 bars(highp vec2 p) {
	highp float vx0 = u[28], pitch = u[29], pw = u[30], top = u[31], vy1 = u[32];
	highp float fi = (p.x - vx0) / pitch;
	if (fi < 0.0 || fi >= 28.0 || p.y < top - 2.0 || p.y > vy1 + 2.0) return vec3(0.0);
	float i = floor(float(fi));
	highp float cx = vx0 + pitch * (i + 0.5);
	float h = u[int(i)];
	highp float span = vy1 - top;
	highp float hh = pw + (span - pw) * h;
	highp float r = pw * 0.5;
	highp float y0 = vy1 - hh + r, y1 = vy1 - r;
	highp float d = length(vec2(p.x - cx, p.y - clamp(p.y, y0, y1))) - r;
	float k = clamp(0.5 - float(d), 0.0, 1.0);
	vec3 c = mod(i, 2.0) > 0.5 ? vec3(u[36], u[37], u[38]) : vec3(u[33], u[34], u[35]);
	return c * u[39] * k;
}

vec3 scene(highp vec2 p, highp vec2 uv) {
	vec3 col = texture2D(t0, uv).rgb;
	if (p.x >= u[65] && p.y >= u[66] && p.x < u[65] + u[67] && p.y < u[66] + u[68])
		col = texture2D(t1, (p - vec2(u[65], u[66])) / vec2(u[67], u[68])).rgb;
	if (p.x >= u[69] && p.y >= u[70] && p.x < u[69] + u[71] && p.y < u[70] + u[72])
		col = texture2D(t2, (p - vec2(u[69], u[70])) / vec2(u[71], u[72])).rgb;
	for (int k = 0; k < 5; k++) {
		float a = u[44 + 5 * k];
		if (a <= 0.01) continue;
		highp vec4 b = vec4(u[40 + 5 * k], u[41 + 5 * k], u[42 + 5 * k], u[43 + 5 * k]);
		if (p.x >= b.x && p.y >= b.y && p.x < b.x + b.z && p.y < b.y + b.w) col = mix(col, vec3(1.0), a);
	}
	vec3 b = bars(p);
	float bk = max(b.r, max(b.g, b.b));
	if (bk > 0.0) col = b + col * (1.0 - clamp(bk / max(u[39], 0.01), 0.0, 1.0) * u[39]);
	return col;
}

void main() {
	highp float s = res.x / u[73];
	highp vec2 p = pixel() / s;
#ifdef LIGHT
	gl_FragColor = vec4(bars(p), 1.0);
#else
	gl_FragColor = vec4(min(scene(p, vtex) + glow(), 1.0), 1.0);
#endif
}
