package com.brotherlogic.recordcollection.integration;

import org.apache.log4j.ConsoleAppender;
import org.apache.log4j.Level;
import org.apache.log4j.Logger;
import org.apache.log4j.PatternLayout;

import com.brotherlogic.recordcollection.TestingSystem;

import org.junit.Before;

public class BaseIntegrationTest {

    @Before
    public void setup() {
        // Set up logging
        ConsoleAppender console = new ConsoleAppender(); // create appender
        String pattern = "[%c] [%d{dd MMM yyyy - hh:mm:ss}] %5p - %m %n";
        console.setLayout(new PatternLayout(pattern));
        console.setThreshold(Level.DEBUG);
        console.activateOptions();

        // add appender to any Logger (here is root)
        Logger.getRootLogger().removeAllAppenders();
        Logger.getRootLogger().addAppender(console);

        // Clean out the testing system
        new TestingSystem().getStorage(true);
    }
}
