void main() {
#ifdef FEED
	gl_FragColor = texture2D(feed, vgl) * u[0];
#else
#ifdef LIGHT
	gl_FragColor = texture2D(feed, vgl);
#else
	highp vec2 p = pixel();
	highp float ground = u[19];
	vec3 col = texture2D(t0, vtex).rgb;
	for (int i = 0; i < 3; i++) {
		float a = u[3 + 6 * i];
		if (a <= 0.0) continue;
		float d = float(length(p - vec2(u[1 + 6 * i], u[2 + 6 * i])) / (res.y * 0.55));
		col += vec3(u[4 + 6 * i], u[5 + 6 * i], u[6 + 6 * i]) * a * max(1.0 - d, 0.0);
	}
	col += texture2D(feed, vgl).rgb;
	vec4 sk = texture2D(t1, vtex);
	col = sk.rgb + min(col, 1.0) * (1.0 - sk.a);
	if (p.y >= ground) {
		highp float sy = ground - (p.y - ground) / 0.26;
		if (sy >= 0.0) col += texture2D(feed, vec2(vgl.x, 1.0 - sy / res.y)).rgb * 0.34;
		float wt = float((p.y - ground) / (res.y - ground));
		col = mix(min(col, 1.0), vec3(2.0, 3.0, 9.0) / 255.0, 0.85 * wt);
	} else if (u[20] > 0.01 && p.y > ground - res.y * 0.2) {
		float k = float((p.y - (ground - res.y * 0.2)) / (res.y * 0.2));
		col += vec3(u[21], u[22], u[23]) * u[20] * 0.6 * k;
	}
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
#endif
}
