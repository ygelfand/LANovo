void main() {
	highp vec2 p = pixel();
	highp vec2 c = vec2(u[1], u[2]) * res;
#ifdef FEED
	vec3 col = texture2D(feed, vgl).rgb * u[0];
	highp vec2 e = p - c;
	highp float cr = cos(u[5]), sr = sin(u[5]);
	highp vec2 l = vec2(cr * e.x + sr * e.y, (-sr * e.x + cr * e.y) / u[4]);
	highp float rl = length(l);
	float w = 2.5 / max(0.3, u[4]) * u[6] * res.y;
	for (int i = 0; i < 3; i++) {
		float a = u[9 + 2 * i];
		if (a <= 0.0) continue;
		highp float rd = u[8 + 2 * i] * res.y;
		float k = clamp(w * 0.5 - abs(float(rl - rd)) + 0.5, 0.0, 1.0);
		col += vec3(u[19], u[20], u[21]) * a * k;
	}
	gl_FragColor = vec4(min(col, 1.0), 1.0);
#else
#ifdef LIGHT
	gl_FragColor = texture2D(feed, vgl);
#else
	vec3 col = texture2D(t0, vtex).rgb + texture2D(feed, vgl).rgb;
	float lv = u[15];
	highp float cr = u[14] * res.y;
	float t = float(length(p - c) / cr);
	if (t < 1.0) {
		vec3 c0 = vec3(u[16], u[17], u[18]);
		vec4 k = t < 0.25 ? mix(vec4(1.0, 1.0, 1.0, 0.42 + 0.35 * lv), vec4(c0, 0.3 + 0.25 * lv), t / 0.25)
			: mix(vec4(c0, 0.3 + 0.25 * lv), vec4(c0, 0.0), (t - 0.25) / 0.75);
		col += k.rgb * k.a;
	}
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
#endif
}
