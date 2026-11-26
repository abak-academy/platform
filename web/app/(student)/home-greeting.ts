export function greetingKey(hour: number) {
  if (hour < 11) return "stu_home_greet_morning";
  if (hour < 15) return "stu_home_greet_afternoon";
  if (hour < 18) return "stu_home_greet_late_afternoon";
  return "stu_home_greet_evening";
}
