package in.tallybridge.app;

final class GsmCalculator {
 private GsmCalculator(){}

 static double calculate(double length,double breadth,double weightGrams,String unit){
  if(length<=0||breadth<=0||weightGrams<=0)throw new IllegalArgumentException("Length, breadth and weight must be greater than zero");
  double factor="cm".equalsIgnoreCase(unit)?0.01:0.0254;
  double areaM2=(length*factor)*(breadth*factor);
  return weightGrams/areaM2;
 }

 static long rounded(double length,double breadth,double weightGrams,String unit){
  return Math.round(calculate(length,breadth,weightGrams,unit));
 }
}
