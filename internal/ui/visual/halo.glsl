const highp float TAU = 6.2831853;

vec3 pal(float p) {
	float j = p * 2.0;
	float k = min(1.0, floor(j));
	float fr = j - k;
	vec3 a = k < 0.5 ? mix(vec3(60.0, 225.0, 255.0), vec3(80.0, 130.0, 255.0), fr) : mix(vec3(80.0, 130.0, 255.0), vec3(160.0, 100.0, 255.0), fr);
	vec3 b = k < 0.5 ? mix(vec3(255.0, 80.0, 190.0), vec3(255.0, 120.0, 120.0), fr) : mix(vec3(255.0, 120.0, 120.0), vec3(255.0, 190.0, 110.0), fr);
	return mix(a, b, u[77]) / 255.0;
}

float bar(highp vec2 d, int q, float width, float px) {
	highp float an = -TAU / 4.0 + float(q) / 72.0 * TAU + u[76];
	highp vec2 dir = vec2(cos(an), sin(an));
	highp float l = u[q];
	highp float s = clamp(dot(d, dir), u[74] - l * 0.35, u[74] + 4.0 * u[75] + l);
	highp float dist = length(d - dir * s);
	return clamp((width * 0.5 - float(dist)) * px + 0.5, 0.0, 1.0);
}

vec3 bars(highp vec2 d, float width, float alpha, float px) {
	highp float rd = length(d);
	if (rd < u[74] - u[93] * 0.35 - width || rd > u[74] + 4.0 * u[75] + u[93] + width) return vec3(0.0);
	highp float an = atan(d.y, d.x);
	highp float k = (an + TAU / 4.0 - u[76]) / TAU * 72.0;
	int q = int(mod(floor(k + 0.5), 72.0));
	vec3 col = vec3(0.0);
	for (int j = -1; j <= 1; j++) {
		int b = q + j;
		if (b < 0) b += 72;
		if (b > 71) b -= 72;
		float c = bar(d, b, width, px);
		if (c > 0.0) {
			float p = float(b < 36 ? b : 72 - b) / 36.0;
			col += pal(p) * alpha * c;
		}
	}
	return col;
}

highp vec2 around(highp vec2 p) {
	return vec2(p.x / res.y - u[72] * res.x / res.y, p.y / res.y - u[73]);
}

void main() {
	highp vec2 p = pixel();
	highp vec2 d = around(p);
	float px = res.y;
	float r0 = u[74], uu = u[75];
#ifdef FEED
	highp float c = cos(u[80]), s = sin(u[80]);
	highp vec2 back = vec2(c * d.x + s * d.y, -s * d.x + c * d.y) / 1.035;
	highp vec2 at = vec2((back.x + u[72] * res.x / res.y) * res.y / res.x, 1.0 - (back.y + u[73]));
	vec3 col = texture2D(feed, at).rgb * u[81];
	col += bars(d, max(1.0 / px, TAU * r0 / 72.0 * 0.34), 0.3, px);
	highp float rd = length(d);
	for (int i = 0; i < 5; i++) {
		float a = u[83 + 2 * i];
		if (a <= 0.0) continue;
		float ag = u[82 + 2 * i];
		highp float rr = r0 + ag * u[92] * 0.3;
		float w = (3.0 + 5.0 * (1.0 - ag)) * uu;
		float k = clamp((w * 0.5 - abs(float(rd - rr))) * px + 0.5, 0.0, 1.0);
		col += pal(0.2) * a * (1.0 - ag) * 0.9 * k;
	}
	gl_FragColor = vec4(min(col, 1.0), 1.0);
#else
#ifdef LIGHT
	gl_FragColor = texture2D(feed, vgl);
#else
	float lv = u[78], ring = u[79];
	highp float rd = length(d);
	vec3 col = vec3(3.0, 4.0, 10.0) / 255.0;
	float amb = max(1.0 - float(rd / (u[92] * 0.7)), 0.0);
	col = mix(col, pal(0.3), (0.08 + 0.12 * lv) * amb);
	col += texture2D(feed, vgl).rgb;
	col += bars(d, max(1.5 / px, TAU * r0 / 72.0 * 0.36), 0.8, px);
	float rw = (2.0 + 3.0 * lv + 4.0 * ring) * uu;
	float rk = clamp((rw * 0.5 - abs(float(rd - (r0 - 6.0 * uu)))) * px + 0.5, 0.0, 1.0);
	col += mix(pal(0.1), vec3(1.0), 0.55) * (0.55 + 0.35 * lv + 0.3 * ring) * rk;
	col = min(col, 1.0);
	float ir = r0 - 8.0 * uu;
	float ik = clamp((ir - float(rd)) * px + 0.5, 0.0, 1.0);
	if (ik > 0.0) {
		float t = float(rd) / ir;
		vec4 ic = t < 0.8 ? mix(vec4(4.0, 6.0, 14.0, 240.0), vec4(6.0, 8.0, 18.0, 230.0), t / 0.8) : mix(vec4(6.0, 8.0, 18.0, 230.0), vec4(10.0, 14.0, 30.0, 153.0), (t - 0.8) / 0.2);
		ic /= 255.0;
		col = mix(col, ic.rgb, ic.a * ik);
	}
	float ck = max(1.0 - float(rd / (r0 * 0.55)), 0.0);
	col += pal(0.5) * (0.1 + 0.3 * lv + 0.2 * ring) * ck;
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
#endif
}
